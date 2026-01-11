package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/qntx/gox/internal/archive"
	"github.com/qntx/gox/internal/ui"
)

// Builder orchestrates cross-compilation using Zig as the C toolchain.
type Builder struct {
	zig    string
	opts   *Options
	stdout io.Writer
	stderr io.Writer
}

// New creates a Builder with default stdout/stderr.
func New(zigPath string, opts *Options) *Builder {
	return &Builder{zig: zigPath, opts: opts, stdout: os.Stdout, stderr: os.Stderr}
}

// NewWithOutput creates a Builder with custom output writers.
func NewWithOutput(zigPath string, opts *Options, stdout, stderr io.Writer) *Builder {
	return &Builder{zig: zigPath, opts: opts, stdout: stdout, stderr: stderr}
}

// Run executes the full build pipeline.
func (b *Builder) Run(ctx context.Context, pkgs []string) error {
	if err := b.setupPackages(ctx); err != nil {
		return fmt.Errorf("packages: %w", err)
	}
	if err := b.setupDirs(); err != nil {
		return fmt.Errorf("dirs: %w", err)
	}
	if err := b.compile(ctx, pkgs); err != nil {
		return err
	}
	if err := b.copyLibs(); err != nil {
		return fmt.Errorf("libs: %w", err)
	}
	if b.opts.Pack {
		if err := b.createArchive(); err != nil {
			return fmt.Errorf("pack: %w", err)
		}
	}
	return nil
}

func (b *Builder) setupPackages(ctx context.Context) error {
	if len(b.opts.Packages) == 0 {
		return nil
	}
	pkgs, err := EnsureAll(ctx, b.opts.Packages)
	if err != nil {
		return err
	}
	inc, lib, bin := CollectPaths(pkgs)
	b.opts.IncludeDirs = append(inc, b.opts.IncludeDirs...)
	b.opts.LibDirs = append(lib, b.opts.LibDirs...)
	b.opts.BinDirs = append(bin, b.opts.BinDirs...)
	return nil
}

func (b *Builder) setupDirs() error {
	out := b.outputPath()
	if out == "" {
		return nil
	}
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if b.opts.Prefix != "" && b.opts.GOOS != "windows" {
		return os.MkdirAll(filepath.Join(b.opts.Prefix, "lib"), 0o755)
	}
	return nil
}

func (b *Builder) compile(ctx context.Context, pkgs []string) error {
	env := b.buildEnv()
	args := b.buildArgs(pkgs)

	ui.Building(fmt.Sprintf("%s/%s", b.opts.GOOS, b.opts.GOARCH))
	if b.opts.Verbose {
		b.logBuild(env, args)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = b.stdout, b.stderr

	if err := cmd.Run(); err != nil {
		ui.BuildFailed()
		return err
	}

	ui.Built(b.outputPath(), time.Since(start))
	return nil
}

func (b *Builder) copyLibs() error {
	if b.opts.Prefix == "" || b.opts.LinkMode.IsStatic() {
		return nil
	}

	if b.opts.GOOS == "windows" {
		if len(b.opts.BinDirs) == 0 {
			return nil
		}
		for _, src := range b.opts.BinDirs {
			if err := copyDir(src, b.opts.Prefix); err != nil {
				return fmt.Errorf("%s: %w", src, err)
			}
		}
		if b.opts.Verbose {
			fmt.Fprintf(os.Stderr, "dlls: %s\n", b.opts.Prefix)
		}
		return nil
	}

	if len(b.opts.LibDirs) == 0 {
		return nil
	}
	dst := filepath.Join(b.opts.Prefix, "lib")
	for _, src := range b.opts.LibDirs {
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
	}
	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "libs: %s\n", dst)
	}
	return nil
}

func (b *Builder) createArchive() error {
	src := b.opts.Prefix
	if src == "" {
		src = b.opts.Output
	}
	if src == "" {
		return fmt.Errorf("--pack requires --output or --prefix")
	}
	path, err := archive.Create(src, b.opts.GOOS, b.opts.GOARCH)
	if err != nil {
		return err
	}
	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "pack: %s\n", path)
	}
	return nil
}

func (b *Builder) buildEnv() []string {
	target := b.opts.ZigTarget()
	env := []string{
		"CGO_ENABLED=1",
		"GOOS=" + b.opts.GOOS,
		"GOARCH=" + b.opts.GOARCH,
		"CC=" + b.zigCC("cc", target),
		"CXX=" + b.zigCC("c++", target),
	}
	if flags := b.cgoFlags(); flags != "" {
		env = append(env, "CGO_CFLAGS="+flags)
	}
	if flags := b.cgoLDFlags(); flags != "" {
		env = append(env, "CGO_LDFLAGS="+flags)
	}
	return env
}

func (b *Builder) buildArgs(pkgs []string) []string {
	args := []string{"build"}
	if out := b.outputPath(); out != "" {
		args = append(args, "-o", out)
	}
	if flags := b.goLDFlags(); flags != "" {
		args = append(args, "-ldflags="+flags)
	}
	args = append(args, b.opts.BuildFlags...)
	if len(pkgs) == 0 {
		return append(args, ".")
	}
	return append(args, pkgs...)
}

func (b *Builder) zigCC(mode, target string) string {
	bin := filepath.Join(b.zig, "zig")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return fmt.Sprintf("%s %s -target %s", bin, mode, target)
}

func (b *Builder) cgoFlags() string {
	flags := []string{"-Wno-macro-redefined"}
	for _, d := range b.opts.IncludeDirs {
		flags = append(flags, "-I"+d)
	}
	return strings.Join(flags, " ")
}

func (b *Builder) cgoLDFlags() string {
	var flags []string
	for _, d := range b.opts.LibDirs {
		flags = append(flags, "-L"+d)
	}
	for _, l := range b.opts.Libs {
		flags = append(flags, "-l"+l)
	}
	if b.opts.LinkMode.IsStatic() {
		flags = append(flags, "-static")
	}
	if rpath := b.rpath(); rpath != "" {
		flags = append(flags, rpath)
	}
	return strings.Join(flags, " ")
}

func (b *Builder) goLDFlags() string {
	var flags []string
	if b.opts.Strip {
		flags = append(flags, "-s", "-w")
	} else if b.opts.GOOS == "darwin" && runtime.GOOS != "darwin" {
		flags = append(flags, "-w")
	}
	switch b.opts.LinkMode {
	case LinkStatic:
		flags = append(flags, "-linkmode=external", `-extldflags "-static"`)
	case LinkDynamic:
		flags = append(flags, "-linkmode=external")
	}
	return strings.Join(flags, " ")
}

func (b *Builder) rpath() string {
	if b.opts.Prefix == "" || b.opts.NoRpath || b.opts.LinkMode.IsStatic() {
		return ""
	}
	switch b.opts.GOOS {
	case "linux", "freebsd", "netbsd":
		return "-Wl,-rpath,$ORIGIN/../lib"
	case "darwin":
		return "-Wl,-rpath,@executable_path/../lib"
	}
	return ""
}

func (b *Builder) outputPath() string {
	if b.opts.Output != "" {
		return b.opts.Output
	}
	if b.opts.Prefix == "" {
		return ""
	}
	name := filepath.Base(b.opts.Prefix)
	if b.opts.GOOS == "windows" {
		return filepath.Join(b.opts.Prefix, name+".exe")
	}
	return filepath.Join(b.opts.Prefix, "bin", name)
}

func (b *Builder) logBuild(env, args []string) {
	if out := b.outputPath(); out != "" {
		fmt.Fprintf(os.Stderr, "out: %s\n", out)
	}
	fmt.Fprintf(os.Stderr, "env: %v\ngo %s\n", env, strings.Join(args, " "))
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())

		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := copySymlink(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath, info.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return copyFile(src, dst, 0)
	}
	_ = os.Remove(dst)
	if os.Symlink(target, dst) == nil {
		return nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(src), target)
	}
	return copyFile(target, dst, 0)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if mode == 0 {
		if fi, err := in.Stat(); err == nil {
			mode = fi.Mode()
		} else {
			mode = 0o644
		}
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if e := out.Close(); err == nil {
		err = e
	}
	return err
}
