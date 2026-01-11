package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/ui"
	"github.com/qntx/gox/internal/zig"
)

type flags struct {
	config   string
	targets  []string
	linkMode string
	parallel bool
	opts     build.Options
}

var (
	bf = flags{}

	buildCmd = &cobra.Command{
		Use:   "build [packages]",
		Short: "Build Go packages with CGO cross-compilation support",
		Long: `Build compiles Go packages using Zig as the C/C++ compiler for CGO.

Configuration can be loaded from gox.toml in the current or parent directories.
CLI flags override config file values.

When --target is not specified and gox.toml exists, all targets are built.
Use --target to build specific targets (comma-separated or repeated).`,
		RunE: runBuild,
	}
)

func init() {
	f := buildCmd.Flags()

	// Config
	f.StringVarP(&bf.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringSliceVarP(&bf.targets, "target", "t", nil, "build targets")

	// Platform
	f.StringVar(&bf.opts.GOOS, "os", "", "target operating system")
	f.StringVar(&bf.opts.GOARCH, "arch", "", "target architecture")

	// Output
	f.StringVarP(&bf.opts.Output, "output", "o", "", "output file path")
	f.StringVar(&bf.opts.Prefix, "prefix", "", "output prefix directory")
	f.BoolVar(&bf.opts.NoRpath, "no-rpath", false, "disable rpath")
	f.BoolVar(&bf.opts.Pack, "pack", false, "create archive")

	// Toolchain
	f.StringVar(&bf.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&bf.linkMode, "linkmode", "", "link mode: static|dynamic|auto")

	// Dependencies
	f.StringSliceVarP(&bf.opts.IncludeDirs, "include", "I", nil, "include directories")
	f.StringSliceVarP(&bf.opts.LibDirs, "lib", "L", nil, "library directories")
	f.StringSliceVarP(&bf.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&bf.opts.Packages, "pkg", nil, "packages to download")

	// Build
	f.StringSliceVar(&bf.opts.BuildFlags, "flags", nil, "additional build flags")
	f.BoolVarP(&bf.opts.Verbose, "verbose", "v", false, "verbose output")
	f.BoolVarP(&bf.parallel, "parallel", "j", false, "parallel builds")

	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	opts, err := loadOptions(cmd)
	if err != nil {
		return err
	}

	if bf.parallel && len(opts) > 1 {
		return buildParallel(cmd, args, opts)
	}
	return buildSequential(cmd, args, opts)
}

func buildSequential(cmd *cobra.Command, args []string, opts []*build.Options) error {
	for i, o := range opts {
		if err := doBuild(cmd, args, o, i, len(opts)); err != nil {
			return err
		}
	}
	return nil
}

func buildParallel(cmd *cobra.Command, args []string, opts []*build.Options) error {
	ui.Info("Building %d targets in parallel...", len(opts))

	type result struct {
		target string
		output string
		err    error
	}

	results := make(chan result, len(opts))
	var wg sync.WaitGroup

	for _, o := range opts {
		wg.Go(func() {
			var buf bytes.Buffer
			target := fmt.Sprintf("%s/%s", o.GOOS, o.GOARCH)
			err := doBuildBuffered(cmd, args, o, &buf)
			results <- result{target: target, output: buf.String(), err: err}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Print results as they complete
	var errs []error
	for r := range results {
		if r.output != "" {
			os.Stderr.WriteString(r.output)
		}
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.target, r.err))
		}
	}

	switch len(errs) {
	case 0:
		ui.Success("All %d targets built successfully", len(opts))
		return nil
	case 1:
		return errs[0]
	default:
		return fmt.Errorf("%d targets failed", len(errs))
	}
}

func doBuild(cmd *cobra.Command, args []string, opts *build.Options, idx, total int) error {
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
		fmt.Fprintf(os.Stderr, "zig: %s\n", zigPath)
	}

	return build.New(zigPath, opts).Run(cmd.Context(), args)
}

func doBuildBuffered(cmd *cobra.Command, args []string, opts *build.Options, buf *bytes.Buffer) error {
	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return err
	}

	zigPath, err := zig.Ensure(cmd.Context(), opts.ZigVersion)
	if err != nil {
		return fmt.Errorf("zig: %w", err)
	}

	return build.NewWithOutput(zigPath, opts, buf, buf).Run(cmd.Context(), args)
}

func loadOptions(cmd *cobra.Command) ([]*build.Options, error) {
	cfg, err := build.LoadConfig(bf.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var opts []*build.Options
	if cfg != nil {
		opts, err = cfg.ToOptions(bf.targets)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		opts = []*build.Options{{}}
	}

	for _, o := range opts {
		mergeFlags(cmd, o)
	}
	return opts, nil
}

func mergeFlags(cmd *cobra.Command, o *build.Options) {
	f := cmd.Flags()

	// Strings
	setIf(f, "os", &o.GOOS, bf.opts.GOOS)
	setIf(f, "arch", &o.GOARCH, bf.opts.GOARCH)
	setIf(f, "output", &o.Output, bf.opts.Output)
	setIf(f, "prefix", &o.Prefix, bf.opts.Prefix)
	setIf(f, "zig-version", &o.ZigVersion, bf.opts.ZigVersion)

	// Slices
	setSliceIf(f, "include", &o.IncludeDirs, bf.opts.IncludeDirs)
	setSliceIf(f, "lib", &o.LibDirs, bf.opts.LibDirs)
	setSliceIf(f, "link", &o.Libs, bf.opts.Libs)
	setSliceIf(f, "pkg", &o.Packages, bf.opts.Packages)
	setSliceIf(f, "flags", &o.BuildFlags, bf.opts.BuildFlags)

	// Bools
	setBoolIf(f, "no-rpath", &o.NoRpath, bf.opts.NoRpath)
	setBoolIf(f, "pack", &o.Pack, bf.opts.Pack)
	setBoolIf(f, "verbose", &o.Verbose, bf.opts.Verbose)

	// LinkMode
	if f.Changed("linkmode") {
		o.LinkMode = build.LinkMode(bf.linkMode)
	}
}

type changedChecker interface{ Changed(string) bool }

func setIf(f changedChecker, name string, dst *string, src string) {
	if f.Changed(name) {
		*dst = src
	}
}

func setSliceIf(f changedChecker, name string, dst *[]string, src []string) {
	if f.Changed(name) {
		*dst = src
	}
}

func setBoolIf(f changedChecker, name string, dst *bool, src bool) {
	if f.Changed(name) {
		*dst = src
	}
}
