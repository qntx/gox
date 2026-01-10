package cmd

import (
	"fmt"
	"os"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/prompt"
	"github.com/qntx/gox/internal/zig"
	"github.com/spf13/cobra"
)

var buildOpts = &build.Options{}

var buildCmd = &cobra.Command{
	Use:   "build [packages]",
	Short: "Build Go packages with CGO cross-compilation support",
	Long:  `Build compiles Go packages using Zig as the C/C++ compiler for CGO.`,
	RunE:  runBuild,
}

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
	f.StringVar(&buildOpts.LinkMode, "linkmode", "", "link mode: static, dynamic, or auto")
	f.StringSliceVar(&buildOpts.BuildFlags, "flags", nil, "additional go build flags")
	f.BoolVar(&buildOpts.Pack, "pack", false, "create archive after build")
	f.BoolVarP(&buildOpts.Interactive, "interactive", "i", false, "interactive mode")
	f.BoolVarP(&buildOpts.Verbose, "verbose", "v", false, "verbose output")
}

func runBuild(cmd *cobra.Command, args []string) error {
	if buildOpts.Interactive || (buildOpts.GOOS == "" && buildOpts.GOARCH == "") {
		opts, err := prompt.Run(buildOpts)
		if err != nil {
			return fmt.Errorf("prompt: %w", err)
		}
		buildOpts = opts
	}

	buildOpts.Normalize()

	if err := buildOpts.Validate(); err != nil {
		return err
	}

	zigPath, err := zig.Ensure(cmd.Context(), buildOpts.ZigVersion)
	if err != nil {
		return fmt.Errorf("zig: %w", err)
	}

	if buildOpts.Verbose {
		fmt.Fprintf(os.Stderr, "using zig: %s\n", zigPath)
	}

	builder := build.New(zigPath, buildOpts)
	return builder.Run(cmd.Context(), args)
}
