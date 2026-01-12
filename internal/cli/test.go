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

type testFlags struct {
	config   string
	target   string
	linkMode string
	opts     build.Options
}

var (
	tFlags  testFlags
	testCmd = &cobra.Command{
		Use:   "test [build flags] [packages] [-- test flags]",
		Short: "Test Go packages with CGO support",
		Long: `Test compiles and runs tests for the named packages using Zig as the C/C++ compiler.

This is equivalent to 'go test' but with Zig configured as the C toolchain,
enabling CGO testing without manual environment setup.

Arguments after -- are passed directly to the test binary.

Configuration can be loaded from gox.toml. When using config, only the target
matching the current platform (or specified by --target) is used.`,
		RunE: runTest,
	}
)

func init() {
	f := testCmd.Flags()

	f.StringVarP(&tFlags.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringVarP(&tFlags.target, "target", "t", "", "target name from config (must match current platform)")
	f.StringVar(&tFlags.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&tFlags.linkMode, "linkmode", "", "link mode: static|dynamic|auto")
	f.StringSliceVarP(&tFlags.opts.IncludeDirs, "include", "I", nil, "include directories")
	f.StringSliceVarP(&tFlags.opts.LibDirs, "lib", "L", nil, "library directories")
	f.StringSliceVarP(&tFlags.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&tFlags.opts.Packages, "pkg", nil, "packages to download")
	f.StringSliceVar(&tFlags.opts.BuildFlags, "flags", nil, "additional build flags")
	f.BoolVarP(&tFlags.opts.Verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	pkgs, testArgs := splitTestArgs(args)

	opts, err := loadTestOptions(cmd)
	if err != nil {
		return err
	}

	if err := validateTestTarget(opts); err != nil {
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

	return build.New(zigPath, opts).GoTest(cmd.Context(), pkgs, testArgs)
}

func splitTestArgs(args []string) (pkgs, testArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func loadTestOptions(cmd *cobra.Command) (*build.Options, error) {
	cfg, err := build.LoadConfig(tFlags.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var opts *build.Options
	if cfg != nil {
		opts, err = selectTestTarget(cfg)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		opts = &build.Options{}
	}

	applyTestFlagOverrides(cmd, opts)
	return opts, nil
}

func selectTestTarget(cfg *build.Config) (*build.Options, error) {
	if tFlags.target != "" {
		all, err := cfg.ToOptions([]string{tFlags.target})
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

func validateTestTarget(opts *build.Options) error {
	goos := opts.GOOS
	goarch := opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		return fmt.Errorf("cannot test %s/%s on %s/%s (cross-testing not supported without --exec)",
			goos, goarch, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

func applyTestFlagOverrides(cmd *cobra.Command, o *build.Options) {
	changed := cmd.Flags().Changed

	if changed("zig-version") {
		o.ZigVersion = tFlags.opts.ZigVersion
	}
	if changed("linkmode") {
		o.LinkMode = build.LinkMode(tFlags.linkMode)
	}
	if changed("include") {
		o.IncludeDirs = tFlags.opts.IncludeDirs
	}
	if changed("lib") {
		o.LibDirs = tFlags.opts.LibDirs
	}
	if changed("link") {
		o.Libs = tFlags.opts.Libs
	}
	if changed("pkg") {
		o.Packages = tFlags.opts.Packages
	}
	if changed("flags") {
		o.BuildFlags = tFlags.opts.BuildFlags
	}
	if changed("verbose") {
		o.Verbose = tFlags.opts.Verbose
	}

	o.Output = ""
	o.Prefix = ""
	o.Pack = false
	o.NoRpath = false
}
