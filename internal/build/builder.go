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

// Builder orchestrates cross-compilation using Zig.
type Builder struct {
	zig    string
	opts   *Options
	stdout io.Writer
	stderr io.Writer
}

// New creates a Builder with stdout/stderr going to os.Stdout/os.Stderr.
func New(zigPath string, opts *Options) *Builder {
	return &Builder{zig: zigPath, opts: opts, stdout: os.Stdout, stderr: os.Stderr}
}

// NewWithOutput creates a Builder with custom output writers.
func NewWithOutput(zigPath string, opts *Options, stdout, stderr io.Writer) *Builder {
	return &Builder{zig: zigPath, opts: opts, stdout: stdout, stderr: stderr}
}

// Run executes: packages → build → libs → pack.
func (b *Builder) Run(ctx context.Context, pkgs []string) error {
	if err := b.setupPackages(ctx); err != nil {
		return fmt.Errorf("packages: %w", err)
	}
	if err := b.setupDirs(); err != nil {
		return fmt.Errorf("dirs: %w", err)
	}
	if err := b.build(ctx, pkgs); err != nil {
		return err
	}
	if err := b.copyLibs(); err != nil {
		return fmt.Errorf("libs: %w", err)
	}
	if b.opts.Pack {
		return b.pack()
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
	inc, lib := CollectPaths(pkgs)
	b.opts.IncludeDirs = append(inc, b.opts.IncludeDirs...)
	b.opts.LibDirs = append(lib, b.opts.LibDirs...)
	return nil
}

func (b *Builder) setupDirs() error {
	out := b.output()
	if out == "" {
		return nil
	}
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if b.opts.Prefix != "" {
		return os.MkdirAll(filepath.Join(b.opts.Prefix, "lib"), 0o755)
	}
	return nil
}

func (b *Builder) build(ctx context.Context, pkgs []string) error {
	env := b.env()
	args := b.args(pkgs)

	ui.Building(fmt.Sprintf("%s/%s", b.opts.GOOS, b.opts.GOARCH))
	if b.opts.Verbose {
		b.log(env, args)
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = b.stdout, b.stderr

	if err := cmd.Run(); err != nil {
		ui.BuildFailed()
		return err
	}

	ui.Built(b.output(), time.Since(start))
	return nil
}

func (b *Builder) copyLibs() error {
	if b.opts.Prefix == "" || b.opts.LinkMode.IsStatic() || len(b.opts.LibDirs) == 0 {
		return nil
	}
	dst := filepath.Join(b.opts.Prefix, "lib")
	for _, src := range b.opts.LibDirs {
		if err := cpDir(src, dst); err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
	}
	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "libs: %s\n", dst)
	}
	return nil
}

func (b *Builder) pack() error {
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

func (b *Builder) env() []string {
	tgt := b.opts.ZigTarget()
	env := []string{
		"CGO_ENABLED=1",
		"GOOS=" + b.opts.GOOS,
		"GOARCH=" + b.opts.GOARCH,
		"CC=" + b.cc("cc", tgt),
		"CXX=" + b.cc("c++", tgt),
	}
	if f := b.cflags(); f != "" {
		env = append(env, "CGO_CFLAGS="+f)
	}
	if f := b.ldflags(); f != "" {
		env = append(env, "CGO_LDFLAGS="+f)
	}
	return env
}

func (b *Builder) args(pkgs []string) []string {
	args := []string{"build"}
	if out := b.output(); out != "" {
		args = append(args, "-o", out)
	}
	if f := b.goflags(); f != "" {
		args = append(args, "-ldflags="+f)
	}
	args = append(args, b.opts.BuildFlags...)
	if len(pkgs) == 0 {
		return append(args, ".")
	}
	return append(args, pkgs...)
}

func (b *Builder) cc(mode, target string) string {
	bin := filepath.Join(b.zig, "zig")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return fmt.Sprintf("%s %s -target %s", bin, mode, target)
}

func (b *Builder) cflags() string {
	var p []string
	// Suppress known harmless warnings
	p = append(p, "-Wno-macro-redefined")
	for _, d := range b.opts.IncludeDirs {
		p = append(p, "-I"+d)
	}
	return strings.Join(p, " ")
}

func (b *Builder) ldflags() string {
	var p []string
	for _, d := range b.opts.LibDirs {
		p = append(p, "-L"+d)
	}
	for _, l := range b.opts.Libs {
		p = append(p, "-l"+l)
	}
	if b.opts.LinkMode.IsStatic() {
		p = append(p, "-static")
	}
	if r := b.rpath(); r != "" {
		p = append(p, r)
	}
	return strings.Join(p, " ")
}

func (b *Builder) goflags() string {
	var f []string
	if b.opts.GOOS == "darwin" && runtime.GOOS != "darwin" {
		f = append(f, "-w")
	}
	switch b.opts.LinkMode {
	case LinkStatic:
		f = append(f, "-linkmode=external", `-extldflags "-static"`)
	case LinkDynamic:
		f = append(f, "-linkmode=external")
	}
	return strings.Join(f, " ")
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

func (b *Builder) output() string {
	if b.opts.Output != "" {
		return b.opts.Output
	}
	if b.opts.Prefix == "" {
		return ""
	}
	name := filepath.Base(b.opts.Prefix)
	if b.opts.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(b.opts.Prefix, "bin", name)
}

func (b *Builder) log(env, args []string) {
	if out := b.output(); out != "" {
		fmt.Fprintf(os.Stderr, "out: %s\n", out)
	}
	fmt.Fprintf(os.Stderr, "env: %v\ngo %s\n", env, strings.Join(args, " "))
}

func cpDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())

		info, err := e.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			err = cpLink(s, d)
		} else {
			err = cpFile(s, d, info.Mode())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func cpLink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return cpFile(src, dst, 0)
	}
	_ = os.Remove(dst)
	if os.Symlink(target, dst) == nil {
		return nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(src), target)
	}
	return cpFile(target, dst, 0)
}

func cpFile(src, dst string, mode os.FileMode) error {
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
