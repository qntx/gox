package cli

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/zig"
	"github.com/spf13/cobra"
)

// buildFlags holds CLI flag values for the build command.
type buildFlags struct {
	config   string
	targets  []string
	linkMode string
	parallel bool
	opts     build.Options
}

var bf buildFlags

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

func init() {
	f := buildCmd.Flags()

	// Config and targets
	f.StringVarP(&bf.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringSliceVarP(&bf.targets, "target", "t", nil, "build targets (comma-separated or repeated)")

	// Output
	f.StringVarP(&bf.opts.Output, "output", "o", "", "output file path")
	f.StringVar(&bf.opts.Prefix, "prefix", "", "output prefix directory (creates bin/lib structure)")
	f.BoolVar(&bf.opts.NoRpath, "no-rpath", false, "disable rpath when using --prefix")
	f.BoolVar(&bf.opts.Pack, "pack", false, "create archive after build")

	// Target platform
	f.StringVar(&bf.opts.GOOS, "os", "", "target operating system")
	f.StringVar(&bf.opts.GOARCH, "arch", "", "target architecture")

	// Toolchain
	f.StringVar(&bf.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&bf.linkMode, "linkmode", "", "link mode: static, dynamic, or auto")

	// Dependencies
	f.StringSliceVarP(&bf.opts.IncludeDirs, "include", "I", nil, "C header include directories")
	f.StringSliceVarP(&bf.opts.LibDirs, "lib", "L", nil, "library search directories")
	f.StringSliceVarP(&bf.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&bf.opts.Packages, "pkg", nil, "packages to download (owner/repo@version/asset or URL)")

	// Build options
	f.StringSliceVar(&bf.opts.BuildFlags, "flags", nil, "additional go build flags")
	f.BoolVarP(&bf.opts.Verbose, "verbose", "v", false, "verbose output")
	f.BoolVarP(&bf.parallel, "parallel", "j", false, "build targets in parallel")

	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	optsList, err := resolveOptions(cmd)
	if err != nil {
		return err
	}

	if bf.parallel && len(optsList) > 1 {
		return runParallel(cmd, args, optsList)
	}
	return runSequential(cmd, args, optsList)
}

func runSequential(cmd *cobra.Command, args []string, optsList []*build.Options) error {
	total := len(optsList)
	for i, opts := range optsList {
		if err := executeBuild(cmd, args, opts, i, total); err != nil {
			return err
		}
	}
	return nil
}

func runParallel(cmd *cobra.Command, args []string, optsList []*build.Options) error {
	type result struct {
		idx  int
		opts *build.Options
		err  error
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []result
	)

	for i, opts := range optsList {
		wg.Go(func() {
			err := executeBuild(cmd, args, opts, i, len(optsList))
			mu.Lock()
			results = append(results, result{i, opts, err})
			mu.Unlock()
		})
	}
	wg.Wait()

	// Collect errors
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", r.opts.GOOS, r.opts.GOARCH, r.err))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("%d targets failed: %v", len(errs), errs)
}

func executeBuild(cmd *cobra.Command, args []string, opts *build.Options, idx, total int) error {
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

func resolveOptions(cmd *cobra.Command) ([]*build.Options, error) {
	cfg, err := build.LoadConfig(bf.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var optsList []*build.Options
	if cfg != nil {
		optsList, err = cfg.ToOptions(bf.targets)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		optsList = []*build.Options{{}}
	}

	for _, opts := range optsList {
		applyFlags(cmd, opts)
	}
	return optsList, nil
}

// applyFlags merges CLI flags into options, overriding config values.
func applyFlags(cmd *cobra.Command, opts *build.Options) {
	f := cmd.Flags()

	// String fields
	applyString(f, "os", &opts.GOOS, bf.opts.GOOS)
	applyString(f, "arch", &opts.GOARCH, bf.opts.GOARCH)
	applyString(f, "output", &opts.Output, bf.opts.Output)
	applyString(f, "prefix", &opts.Prefix, bf.opts.Prefix)
	applyString(f, "zig-version", &opts.ZigVersion, bf.opts.ZigVersion)

	// Slice fields
	applySlice(f, "include", &opts.IncludeDirs, bf.opts.IncludeDirs)
	applySlice(f, "lib", &opts.LibDirs, bf.opts.LibDirs)
	applySlice(f, "link", &opts.Libs, bf.opts.Libs)
	applySlice(f, "pkg", &opts.Packages, bf.opts.Packages)
	applySlice(f, "flags", &opts.BuildFlags, bf.opts.BuildFlags)

	// Bool fields
	applyBool(f, "no-rpath", &opts.NoRpath, bf.opts.NoRpath)
	applyBool(f, "pack", &opts.Pack, bf.opts.Pack)
	applyBool(f, "verbose", &opts.Verbose, bf.opts.Verbose)

	// LinkMode
	if f.Changed("linkmode") {
		opts.LinkMode = build.LinkMode(bf.linkMode)
	}
}

type flagSet interface {
	Changed(string) bool
}

func applyString(f flagSet, name string, dst *string, src string) {
	if f.Changed(name) {
		*dst = src
	}
}

func applySlice(f flagSet, name string, dst *[]string, src []string) {
	if f.Changed(name) {
		*dst = src
	}
}

func applyBool(f flagSet, name string, dst *bool, src bool) {
	if f.Changed(name) {
		*dst = src
	}
}
