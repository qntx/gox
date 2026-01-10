package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/tui"
	"github.com/qntx/gox/internal/zig"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [packages]",
	Short: "Build Go packages with CGO cross-compilation support",
	Long: `Build compiles Go packages using Zig as the C/C++ compiler for CGO.

Configuration can be loaded from gox.toml in the current or parent directories.
CLI flags override config file values.

When --target is not specified and gox.toml exists, all targets are built.
Use --target to build specific targets (comma-separated or repeated).`,
	RunE: runBuild,
}

var (
	configFile  string
	targetNames []string
	cliOpts     build.Options
	linkMode    string
)

func init() {
	f := buildCmd.Flags()
	f.StringVarP(&configFile, "config", "c", "", "config file path (default: gox.toml)")
	f.StringSliceVarP(&targetNames, "target", "t", nil, "build targets (comma-separated or repeated)")
	f.StringVarP(&cliOpts.Output, "output", "o", "", "output file path")
	f.StringVar(&cliOpts.Prefix, "prefix", "", "output prefix directory (creates bin/lib structure)")
	f.BoolVar(&cliOpts.NoRpath, "no-rpath", false, "disable rpath when using --prefix")
	f.StringVar(&cliOpts.GOOS, "os", "", "target operating system")
	f.StringVar(&cliOpts.GOARCH, "arch", "", "target architecture")
	f.StringVar(&cliOpts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringSliceVarP(&cliOpts.IncludeDirs, "include", "I", nil, "C header include directories")
	f.StringSliceVarP(&cliOpts.LibDirs, "lib", "L", nil, "library search directories")
	f.StringSliceVarP(&cliOpts.Libs, "link", "l", nil, "libraries to link")
	f.StringVar(&linkMode, "linkmode", "", "link mode: static, dynamic, or auto")
	f.StringSliceVar(&cliOpts.BuildFlags, "flags", nil, "additional go build flags")
	f.BoolVar(&cliOpts.Pack, "pack", false, "create archive after build")
	f.BoolVarP(&cliOpts.Interactive, "interactive", "i", false, "interactive mode")
	f.BoolVarP(&cliOpts.Verbose, "verbose", "v", false, "verbose output")
}

func runBuild(cmd *cobra.Command, args []string) error {
	optsList, err := loadOptions(cmd)
	if err != nil {
		return err
	}

	for i, opts := range optsList {
		if err := runSingleBuild(cmd, args, opts, i, len(optsList)); err != nil {
			return err
		}
	}
	return nil
}

func runSingleBuild(cmd *cobra.Command, args []string, opts *build.Options, idx, total int) error {
	if opts.Interactive || (opts.GOOS == "" && opts.GOARCH == "") {
		p, err := tui.SelectTarget(opts)
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		opts = p
	}

	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return err
	}

	zigPath, err := zig.Ensure(cmd.Context(), opts.ZigVersion)
	if err != nil {
		return fmt.Errorf("zig: %w", err)
	}

	if total > 1 {
		fmt.Fprintf(os.Stderr, "\n[%d/%d] %s/%s\n", idx+1, total, opts.GOOS, opts.GOARCH)
	}
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "using zig: %s\n", zigPath)
	}

	return build.New(zigPath, opts).Run(cmd.Context(), args)
}

func loadOptions(cmd *cobra.Command) ([]*build.Options, error) {
	cfg, err := build.LoadConfig(configFile)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var optsList []*build.Options
	if cfg != nil {
		optsList, err = cfg.ToOptions(targetNames)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		optsList = []*build.Options{{}}
	}

	for _, opts := range optsList {
		mergeCliFlags(cmd, opts)
	}
	return optsList, nil
}

func mergeCliFlags(cmd *cobra.Command, opts *build.Options) {
	flags := cmd.Flags()

	if flags.Changed("os") {
		opts.GOOS = cliOpts.GOOS
	}
	if flags.Changed("arch") {
		opts.GOARCH = cliOpts.GOARCH
	}
	if flags.Changed("output") {
		opts.Output = cliOpts.Output
	}
	if flags.Changed("prefix") {
		opts.Prefix = cliOpts.Prefix
	}
	if flags.Changed("no-rpath") {
		opts.NoRpath = cliOpts.NoRpath
	}
	if flags.Changed("zig-version") {
		opts.ZigVersion = cliOpts.ZigVersion
	}
	if flags.Changed("include") {
		opts.IncludeDirs = cliOpts.IncludeDirs
	}
	if flags.Changed("lib") {
		opts.LibDirs = cliOpts.LibDirs
	}
	if flags.Changed("link") {
		opts.Libs = cliOpts.Libs
	}
	if flags.Changed("linkmode") {
		opts.LinkMode = build.LinkMode(linkMode)
	}
	if flags.Changed("flags") {
		opts.BuildFlags = cliOpts.BuildFlags
	}
	if flags.Changed("pack") {
		opts.Pack = cliOpts.Pack
	}
	if flags.Changed("interactive") {
		opts.Interactive = cliOpts.Interactive
	}
	if flags.Changed("verbose") {
		opts.Verbose = cliOpts.Verbose
	}
}
