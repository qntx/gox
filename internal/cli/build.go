package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/ui"
	"github.com/qntx/gox/internal/zig"
)

type buildFlags struct {
	config   string
	targets  []string
	linkMode string
	parallel bool
	opts     build.Options
}

var (
	flags    buildFlags
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

	f.StringVarP(&flags.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringSliceVarP(&flags.targets, "target", "t", nil, "build targets")
	f.StringVar(&flags.opts.GOOS, "os", "", "target operating system")
	f.StringVar(&flags.opts.GOARCH, "arch", "", "target architecture")
	f.StringVarP(&flags.opts.Output, "output", "o", "", "output file path")
	f.StringVar(&flags.opts.Prefix, "prefix", "", "output prefix directory")
	f.StringVar(&flags.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&flags.linkMode, "linkmode", "", "link mode: static|dynamic|auto")
	f.StringSliceVarP(&flags.opts.IncludeDirs, "include", "I", nil, "include directories")
	f.StringSliceVarP(&flags.opts.LibDirs, "lib", "L", nil, "library directories")
	f.StringSliceVarP(&flags.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&flags.opts.Packages, "pkg", nil, "packages to download")
	f.StringSliceVar(&flags.opts.BuildFlags, "flags", nil, "additional build flags")
	f.BoolVar(&flags.opts.NoRpath, "no-rpath", false, "disable rpath")
	f.BoolVar(&flags.opts.Pack, "pack", false, "create archive")
	f.BoolVarP(&flags.opts.Strip, "strip", "s", false, "strip symbols (-ldflags=\"-s -w\")")
	f.BoolVarP(&flags.opts.Verbose, "verbose", "v", false, "verbose output")
	f.BoolVarP(&flags.parallel, "parallel", "j", false, "parallel builds")

	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	opts, err := loadBuildOptions(cmd)
	if err != nil {
		return err
	}
	if flags.parallel && len(opts) > 1 {
		return runParallel(cmd, args, opts)
	}
	return runSequential(cmd, args, opts)
}

func runSequential(cmd *cobra.Command, args []string, opts []*build.Options) error {
	for i, o := range opts {
		if err := executeBuild(cmd, args, o, i, len(opts)); err != nil {
			return err
		}
	}
	return nil
}

func runParallel(cmd *cobra.Command, args []string, opts []*build.Options) error {
	ui.Header(fmt.Sprintf("Building %d targets", len(opts)))

	if err := preloadPackages(cmd.Context(), opts); err != nil {
		return err
	}

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
			err := executeBuildBuffered(cmd, args, o, &buf)
			results <- result{
				target: fmt.Sprintf("%s/%s", o.GOOS, o.GOARCH),
				output: buf.String(),
				err:    err,
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errs []error
	for r := range results {
		if r.output != "" {
			fmt.Print(r.output)
		}
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.target, r.err))
		}
	}

	if len(errs) == 0 {
		ui.Success("All %d targets built", len(opts))
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return fmt.Errorf("%d targets failed", len(errs))
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

	ui.Target(idx, total, opts.GOOS, opts.GOARCH)
	if opts.Verbose {
		ui.Label("zig", zigPath)
	}

	return build.New(zigPath, opts).Run(cmd.Context(), args)
}

func executeBuildBuffered(cmd *cobra.Command, args []string, opts *build.Options, buf *bytes.Buffer) error {
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

func loadBuildOptions(cmd *cobra.Command) ([]*build.Options, error) {
	cfg, err := build.LoadConfig(flags.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var opts []*build.Options
	if cfg != nil {
		opts, err = cfg.ToOptions(flags.targets)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		opts = []*build.Options{{}}
	}

	for _, o := range opts {
		applyFlagOverrides(cmd, o)
	}
	return opts, nil
}

func applyFlagOverrides(cmd *cobra.Command, o *build.Options) {
	changed := cmd.Flags().Changed

	if changed("os") {
		o.GOOS = flags.opts.GOOS
	}
	if changed("arch") {
		o.GOARCH = flags.opts.GOARCH
	}
	if changed("output") {
		o.Output = flags.opts.Output
	}
	if changed("prefix") {
		o.Prefix = flags.opts.Prefix
	}
	if changed("zig-version") {
		o.ZigVersion = flags.opts.ZigVersion
	}
	if changed("linkmode") {
		o.LinkMode = build.LinkMode(flags.linkMode)
	}
	if changed("include") {
		o.IncludeDirs = flags.opts.IncludeDirs
	}
	if changed("lib") {
		o.LibDirs = flags.opts.LibDirs
	}
	if changed("link") {
		o.Libs = flags.opts.Libs
	}
	if changed("pkg") {
		o.Packages = flags.opts.Packages
	}
	if changed("flags") {
		o.BuildFlags = flags.opts.BuildFlags
	}
	if changed("no-rpath") {
		o.NoRpath = flags.opts.NoRpath
	}
	if changed("pack") {
		o.Pack = flags.opts.Pack
	}
	if changed("strip") {
		o.Strip = flags.opts.Strip
	}
	if changed("verbose") {
		o.Verbose = flags.opts.Verbose
	}
}

func preloadPackages(ctx context.Context, opts []*build.Options) error {
	seen := make(map[string]bool)
	var pkgs []string
	for _, o := range opts {
		for _, pkg := range o.Packages {
			if !seen[pkg] {
				seen[pkg] = true
				pkgs = append(pkgs, pkg)
			}
		}
	}
	if len(pkgs) == 0 {
		return nil
	}
	_, err := build.EnsureAll(ctx, pkgs)
	return err
}
