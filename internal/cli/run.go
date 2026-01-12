package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/qntx/gox/internal/build"
	"github.com/qntx/gox/internal/ui"
	"github.com/qntx/gox/internal/zig"
)

type runFlags struct {
	config   string
	target   string
	linkMode string
	exec     string
	opts     build.Options
}

var (
	rFlags runFlags
	runCmd = &cobra.Command{
		Use:   "run [build flags] [package] [-- arguments...]",
		Short: "Compile and run Go package with CGO support",
		Long: `Run compiles and runs the named main Go package using Zig as the C/C++ compiler.

The package is compiled to a temporary binary which is executed immediately.
Arguments after -- are passed to the compiled program.

Configuration can be loaded from gox.toml. When using config, only the target
matching the current platform (or specified by --target) is used.

Note: Cross-compilation is not supported for run. The target OS and architecture
must match the current system.`,
		RunE:               runRun,
		DisableFlagParsing: false,
	}
)

func init() {
	f := runCmd.Flags()

	f.StringVarP(&rFlags.config, "config", "c", "", "config file path (default: gox.toml)")
	f.StringVarP(&rFlags.target, "target", "t", "", "target name from config (must match current platform)")
	f.StringVar(&rFlags.exec, "exec", "", "execute binary using specified program")
	f.StringVar(&rFlags.opts.ZigVersion, "zig-version", "", "zig compiler version")
	f.StringVar(&rFlags.linkMode, "linkmode", "", "link mode: static|dynamic|auto")
	f.StringSliceVarP(&rFlags.opts.IncludeDirs, "include", "I", nil, "include directories")
	f.StringSliceVarP(&rFlags.opts.LibDirs, "lib", "L", nil, "library directories")
	f.StringSliceVarP(&rFlags.opts.Libs, "link", "l", nil, "libraries to link")
	f.StringSliceVar(&rFlags.opts.Packages, "pkg", nil, "packages to download")
	f.StringSliceVar(&rFlags.opts.BuildFlags, "flags", nil, "additional build flags")
	f.BoolVarP(&rFlags.opts.Verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	pkgs, progArgs := splitRunArgs(args)

	opts, err := loadRunOptions(cmd)
	if err != nil {
		return err
	}

	if err := validateRunTarget(opts); err != nil {
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

	if rFlags.exec != "" {
		return runWithExec(cmd, pkgs, progArgs, opts, zigPath)
	}

	return build.New(zigPath, opts).GoRun(cmd.Context(), pkgs, progArgs)
}

func runWithExec(cmd *cobra.Command, pkgs, progArgs []string, opts *build.Options, zigPath string) error {
	tmpDir, err := os.MkdirTemp("", "gox-run-*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binName := "main"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	opts.Output = tmpDir + string(os.PathSeparator) + binName

	if opts.Verbose {
		ui.Label("output", opts.Output)
	}

	if err := build.New(zigPath, opts).Run(cmd.Context(), pkgs); err != nil {
		return err
	}

	return executeProgram(opts.Output, progArgs, rFlags.exec, opts.Verbose)
}

func splitRunArgs(args []string) (pkgs, progArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func loadRunOptions(cmd *cobra.Command) (*build.Options, error) {
	cfg, err := build.LoadConfig(rFlags.config)
	if err != nil && !errors.Is(err, build.ErrConfigNotFound) {
		return nil, fmt.Errorf("config: %w", err)
	}

	var opts *build.Options
	if cfg != nil {
		opts, err = selectRunTarget(cfg)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	} else {
		opts = &build.Options{}
	}

	applyRunFlagOverrides(cmd, opts)
	return opts, nil
}

func selectRunTarget(cfg *build.Config) (*build.Options, error) {
	if rFlags.target != "" {
		all, err := cfg.ToOptions([]string{rFlags.target})
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

func validateRunTarget(opts *build.Options) error {
	goos := opts.GOOS
	goarch := opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	if goos != runtime.GOOS || goarch != runtime.GOARCH {
		return fmt.Errorf("cannot run %s/%s binary on %s/%s (cross-execution not supported)",
			goos, goarch, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

func applyRunFlagOverrides(cmd *cobra.Command, o *build.Options) {
	changed := cmd.Flags().Changed

	if changed("zig-version") {
		o.ZigVersion = rFlags.opts.ZigVersion
	}
	if changed("linkmode") {
		o.LinkMode = build.LinkMode(rFlags.linkMode)
	}
	if changed("include") {
		o.IncludeDirs = rFlags.opts.IncludeDirs
	}
	if changed("lib") {
		o.LibDirs = rFlags.opts.LibDirs
	}
	if changed("link") {
		o.Libs = rFlags.opts.Libs
	}
	if changed("pkg") {
		o.Packages = rFlags.opts.Packages
	}
	if changed("flags") {
		o.BuildFlags = rFlags.opts.BuildFlags
	}
	if changed("verbose") {
		o.Verbose = rFlags.opts.Verbose
	}

	o.Output = ""
	o.Prefix = ""
	o.Pack = false
	o.NoRpath = false
}

func executeProgram(binPath string, args []string, execProg string, verbose bool) error {
	var cmd *exec.Cmd
	if execProg != "" {
		cmdArgs := append([]string{binPath}, args...)
		cmd = exec.Command(execProg, cmdArgs...)
	} else {
		cmd = exec.Command(binPath, args...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	go func() {
		sig := <-sigCh
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
		}
	}()

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}
