package cli

import (
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
	Long:  `Build compiles Go packages using Zig as the C/C++ compiler for CGO.`,
	RunE:  runBuild,
}

var buildOpts build.Options
var linkModeStr string

func init() {
	f := buildCmd.Flags()
	f.StringVarP(&buildOpts.Output, "output", "o", "", "output file path")
	f.StringVar(&buildOpts.Prefix, "prefix", "", "output prefix directory (creates bin/lib structure)")
	f.BoolVar(&buildOpts.NoRpath, "no-rpath", false, "disable rpath when using --prefix")
	f.StringVar(&buildOpts.GOOS, "os", "", "target operating system")
	f.StringVar(&buildOpts.GOARCH, "arch", "", "target architecture")
	f.StringVar(&buildOpts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringSliceVarP(&buildOpts.IncludeDirs, "include", "I", nil, "C header include directories")
	f.StringSliceVarP(&buildOpts.LibDirs, "lib", "L", nil, "library search directories")
	f.StringSliceVarP(&buildOpts.Libs, "link", "l", nil, "libraries to link")
	f.StringVar(&linkModeStr, "linkmode", "", "link mode: static, dynamic, or auto")
	f.StringSliceVar(&buildOpts.BuildFlags, "flags", nil, "additional go build flags")
	f.BoolVar(&buildOpts.Pack, "pack", false, "create archive after build")
	f.BoolVarP(&buildOpts.Interactive, "interactive", "i", false, "interactive mode")
	f.BoolVarP(&buildOpts.Verbose, "verbose", "v", false, "verbose output")
}

func runBuild(cmd *cobra.Command, args []string) error {
	opts := buildOpts
	opts.LinkMode = build.LinkMode(linkModeStr)

	if opts.Interactive || (opts.GOOS == "" && opts.GOARCH == "") {
		p, err := tui.SelectTarget(&opts)
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		opts = *p
	}

	opts.Normalize()
	if err := opts.Validate(); err != nil {
		return err
	}

	zigPath, err := zig.Ensure(cmd.Context(), opts.ZigVersion)
	if err != nil {
		return fmt.Errorf("zig: %w", err)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "using zig: %s\n", zigPath)
	}

	return build.New(zigPath, &opts).Run(cmd.Context(), args)
}
