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

	"github.com/qntx/gox/internal/archive"
)

const defaultPerm = 0o755

// Builder orchestrates cross-compilation using Zig as the C/C++ toolchain.
type Builder struct {
	zigPath string
	opts    *Options
}

// New creates a Builder with the specified Zig path and build options.
func New(zigPath string, opts *Options) *Builder {
	return &Builder{zigPath: zigPath, opts: opts}
}

// Run executes the complete build pipeline: packages → build → libs → pack.
func (b *Builder) Run(ctx context.Context, packages []string) error {
	if err := b.ensurePackages(ctx); err != nil {
		return fmt.Errorf("packages: %w", err)
	}
	if err := b.ensureOutputDir(); err != nil {
		return fmt.Errorf("output dir: %w", err)
	}

	if err := b.execBuild(ctx, packages); err != nil {
		return err
	}

	if err := b.copyLibs(); err != nil {
		return fmt.Errorf("copy libs: %w", err)
	}
	if b.opts.Pack {
		return b.createArchive()
	}
	return nil
}

// execBuild runs the go build command with configured environment.
func (b *Builder) execBuild(ctx context.Context, packages []string) error {
	env := b.buildEnv()
	args := b.buildArgs(packages)

	if b.opts.Verbose {
		b.logBuild(env, args)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func (b *Builder) logBuild(env, args []string) {
	if out := b.outputPath(); out != "" {
		fmt.Fprintf(os.Stderr, "output: %s\n", out)
	}
	fmt.Fprintf(os.Stderr, "env: %v\ngo %s\n", env, strings.Join(args, " "))
}

func (b *Builder) ensurePackages(ctx context.Context) error {
	if len(b.opts.Packages) == 0 {
		return nil
	}
	pkgs, err := EnsurePackages(ctx, b.opts.Packages)
	if err != nil {
		return err
	}
	inc, lib := CollectPackagePaths(pkgs)
	b.opts.IncludeDirs = append(inc, b.opts.IncludeDirs...)
	b.opts.LibDirs = append(lib, b.opts.LibDirs...)
	return nil
}

func (b *Builder) ensureOutputDir() error {
	out := b.outputPath()
	if out == "" {
		return nil
	}
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, defaultPerm); err != nil {
			return err
		}
	}
	if b.opts.Prefix != "" {
		return os.MkdirAll(filepath.Join(b.opts.Prefix, "lib"), defaultPerm)
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
	if cflags := b.cgoFlags("-I", b.opts.IncludeDirs); cflags != "" {
		env = append(env, "CGO_CFLAGS="+cflags)
	}
	if ldflags := b.cgoLdflags(); ldflags != "" {
		env = append(env, "CGO_LDFLAGS="+ldflags)
	}
	return env
}

func (b *Builder) cgoLdflags() string {
	var parts []string
	for _, dir := range b.opts.LibDirs {
		parts = append(parts, "-L"+dir)
	}
	for _, lib := range b.opts.Libs {
		parts = append(parts, "-l"+lib)
	}
	if b.opts.LinkMode.IsStatic() {
		parts = append(parts, "-static")
	}
	if rpath := b.rpathFlag(); rpath != "" {
		parts = append(parts, rpath)
	}
	return strings.Join(parts, " ")
}

func (b *Builder) cgoFlags(prefix string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = prefix + item
	}
	return strings.Join(parts, " ")
}

func (b *Builder) rpathFlag() string {
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

func (b *Builder) buildArgs(packages []string) []string {
	args := []string{"build"}
	if out := b.outputPath(); out != "" {
		args = append(args, "-o", out)
	}
	if ldflags := b.goLdflags(); ldflags != "" {
		args = append(args, "-ldflags="+ldflags)
	}
	args = append(args, b.opts.BuildFlags...)
	if len(packages) == 0 {
		return append(args, ".")
	}
	return append(args, packages...)
}

func (b *Builder) goLdflags() string {
	var flags []string
	// Cross-compiling to darwin requires -w to avoid dsymutil issues
	if b.opts.GOOS == "darwin" && runtime.GOOS != "darwin" {
		flags = append(flags, "-w")
	}
	switch b.opts.LinkMode {
	case LinkModeStatic:
		flags = append(flags, "-linkmode=external", `-extldflags "-static"`)
	case LinkModeDynamic:
		flags = append(flags, "-linkmode=external")
	}
	return strings.Join(flags, " ")
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
		name += ".exe"
	}
	return filepath.Join(b.opts.Prefix, "bin", name)
}

func (b *Builder) zigCC(mode, target string) string {
	zig := filepath.Join(b.zigPath, "zig")
	if runtime.GOOS == "windows" {
		zig += ".exe"
	}
	return fmt.Sprintf("%s %s -target %s", zig, mode, target)
}

func (b *Builder) copyLibs() error {
	if b.opts.Prefix == "" || b.opts.LinkMode.IsStatic() || len(b.opts.LibDirs) == 0 {
		return nil
	}
	dest := filepath.Join(b.opts.Prefix, "lib")
	for _, src := range b.opts.LibDirs {
		if err := copyDir(src, dest); err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
	}
	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "libs: %s\n", dest)
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
		return fmt.Errorf("archive: %w", err)
	}
	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "archive: %s\n", path)
	}
	return nil
}

// copyDir copies all files (not subdirs) from src to dst.
// Preserves symlinks on Unix; resolves them on Windows.
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
			err = copySymlink(srcPath, dstPath)
		} else {
			err = copyFile(srcPath, dstPath, info.Mode())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return copyFile(src, dst, 0) // fallback: not a symlink
	}
	_ = os.Remove(dst)
	if os.Symlink(target, dst) == nil {
		return nil
	}
	// Windows fallback: copy resolved target
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
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	return err
}
