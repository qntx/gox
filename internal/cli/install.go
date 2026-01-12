package cli

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/ui"
	"github.com/qntx/gox/internal/zig"
)

type installFlags struct {
	config   string
	target   string
	linkMode string
	opts     build.Options
}

var (
	iFlags     installFlags
	installCmd = &cobra.Command{
		Use:   "install [build flags] [packages]",
		Short: "Compile and install Go packages with CGO support",
		Long: `Install compiles and installs packages using Zig as the C/C++ compiler.

Executables are installed in the directory named by the GOBIN environment
variable, which defaults to $GOPATH/bin or $HOME/go/bin.

This is equivalent to 'go install' but with Zig configured as the C toolchain,
enabling CGO compilation without manual environment setup.

Configuration can be loaded from gox.toml. When using config, only the target
matching the current platform (or specified by --target) is used.`,
		RunE: runInstall,
	}
)

func init() {
	f := installCmd.Flags()

	f.StringVarP(&iFlags.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringVarP(&iFlags.target, "target", "t", "", "target name from config (must match current platform)")
	f.StringVar(&iFlags.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&iFlags.linkMode, "linkmode", "", "link mode: static|dynamic|auto")
	f.StringSliceVarP(&iFlags.opts.IncludeDirs, "include", "I", nil, "include directories")
	f.StringSliceVarP(&iFlags.opts.LibDirs, "lib", "L", nil, "library directories")
	f.StringSliceVarP(&iFlags.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&iFlags.opts.Packages, "pkg", nil, "packages to download")
	f.StringSliceVar(&iFlags.opts.BuildFlags, "flags", nil, "additional build flags")
	f.BoolVarP(&iFlags.opts.Strip, "strip", "s", false, "strip symbols (-ldflags=\"-s -w\")")
	f.BoolVarP(&iFlags.opts.Verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	opts, err := loadInstallOptions(cmd)
	if err != nil {
		return err
	}

	if err := validateInstallTarget(opts); err != nil {
		return err
	}

	opts.Normalize()

	zigPath, err := zig.Ensure(cmd.Context(), opts.ZigVersion)
	if err != nil {
		return fmt.Errorf("zig: %w", err)
	}

	if opts.Verbose {
		ui.Label("zig", zigPath)
	}

	return build.New(zigPath, opts).GoInstall(cmd.Context(), args)
}

func loadInstallOptions(cmd *cobra.Command) (*build.Options, error) {
	cfg, err := build.LoadConfig(iFlags.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var opts *build.Options
	if cfg != nil {
		opts, err = selectInstallTarget(cfg)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		opts = &build.Options{}
	}

	applyInstallFlagOverrides(cmd, opts)
	return opts, nil
}

func selectInstallTarget(cfg *build.Config) (*build.Options, error) {
	if iFlags.target != "" {
		all, err := cfg.ToOptions([]string{iFlags.target})
		if err != nil {
			return nil, err
		}
		return all[0], nil
	}

	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		tOS, tArch := t.OS, t.Arch
		if tOS == "" {
			tOS = runtime.GOOS
		}
		if tArch == "" {
			tArch = runtime.GOARCH
		}
		if tOS == runtime.GOOS && tArch == runtime.GOARCH {
			all, err := cfg.ToOptions([]string{t.Name})
			if err != nil {
				return nil, err
			}
			return all[0], nil
		}
	}

	all, err := cfg.ToOptions(nil)
	if err != nil {
		return nil, err
	}
	if len(all) > 0 {
		return all[0], nil
	}
	return &build.Options{}, nil
}

func validateInstallTarget(opts *build.Options) error {
	goos := opts.GOOS
	goarch := opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		return fmt.Errorf("cannot install %s/%s binary on %s/%s (cross-installation not supported)",
			goos, goarch, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

func applyInstallFlagOverrides(cmd *cobra.Command, o *build.Options) {
	changed := cmd.Flags().Changed

	if changed("zig-version") {
		o.ZigVersion = iFlags.opts.ZigVersion
	}
	if changed("linkmode") {
		o.LinkMode = build.LinkMode(iFlags.linkMode)
	}
	if changed("include") {
		o.IncludeDirs = iFlags.opts.IncludeDirs
	}
	if changed("lib") {
		o.LibDirs = iFlags.opts.LibDirs
	}
	if changed("link") {
		o.Libs = iFlags.opts.Libs
	}
	if changed("pkg") {
		o.Packages = iFlags.opts.Packages
	}
	if changed("flags") {
		o.BuildFlags = iFlags.opts.BuildFlags
	}
	if changed("strip") {
		o.Strip = iFlags.opts.Strip
	}
	if changed("verbose") {
		o.Verbose = iFlags.opts.Verbose
	}

	o.Output = ""
	o.Prefix = ""
	o.Pack = false
	o.NoRpath = false
}
