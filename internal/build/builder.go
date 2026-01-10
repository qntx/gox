package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/qntx/gox/internal/archive"
)

const (
	dirPerm   = 0o755
	exeSuffix = ".exe"
)

// Builder executes cross-compilation builds using Zig as the C toolchain.
type Builder struct {
	zigPath string
	opts    *Options
}

// New creates a Builder with the given Zig installation path and options.
func New(zigPath string, opts *Options) *Builder {
	return &Builder{zigPath: zigPath, opts: opts}
}

// Run executes the build for the specified Go packages.
func (b *Builder) Run(ctx context.Context, packages []string) error {
	if err := b.ensurePackages(ctx); err != nil {
		return fmt.Errorf("ensure packages: %w", err)
	}

	if err := b.ensureOutputDir(); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	env := b.buildEnv()
	args := b.buildArgs(packages)

	if b.opts.Verbose {
		b.logVerbose(env, args)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if b.opts.Pack {
		return b.pack()
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

	archivePath, err := archive.Create(src, b.opts.GOOS, b.opts.GOARCH)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "archive: %s\n", archivePath)
	}
	return nil
}

func (b *Builder) logVerbose(env, args []string) {
	if output := b.resolveOutput(); output != "" {
		fmt.Fprintf(os.Stderr, "output: %s\n", output)
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
	output := b.resolveOutput()
	if output == "" {
		return nil
	}

	if dir := filepath.Dir(output); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	if b.opts.Prefix != "" {
		libDir := filepath.Join(b.opts.Prefix, "lib")
		if err := os.MkdirAll(libDir, dirPerm); err != nil {
			return fmt.Errorf("mkdir %s: %w", libDir, err)
		}
	}
	return nil
}

func (b *Builder) buildEnv() []string {
	target := b.opts.ZigTarget()
	env := []string{
		"CGO_ENABLED=1",
		"GOOS=" + b.opts.GOOS,
		"GOARCH=" + b.opts.GOARCH,
		"CC=" + b.zigCmd("cc", target),
		"CXX=" + b.zigCmd("c++", target),
	}

	if cflags := b.cgoCflags(); cflags != "" {
		env = append(env, "CGO_CFLAGS="+cflags)
	}
	if ldflags := b.cgoLdflags(); ldflags != "" {
		env = append(env, "CGO_LDFLAGS="+ldflags)
	}
	return env
}

func (b *Builder) cgoCflags() string {
	return joinPrefixed("-I", b.opts.IncludeDirs)
}

func (b *Builder) cgoLdflags() string {
	var parts []string
	parts = appendPrefixed(parts, "-L", b.opts.LibDirs)
	parts = appendPrefixed(parts, "-l", b.opts.Libs)

	if b.opts.LinkMode.IsStatic() {
		parts = append(parts, "-static")
	}
	if rpath := b.rpathFlag(); rpath != "" {
		parts = append(parts, rpath)
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
	default:
		return ""
	}
}

func (b *Builder) buildArgs(packages []string) []string {
	args := []string{"build"}

	if output := b.resolveOutput(); output != "" {
		args = append(args, "-o", output)
	}
	if ldflags := b.ldflags(); ldflags != "" {
		args = append(args, "-ldflags="+ldflags)
	}

	args = append(args, b.opts.BuildFlags...)

	if len(packages) > 0 {
		return append(args, packages...)
	}
	return append(args, ".")
}

func (b *Builder) ldflags() string {
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

func (b *Builder) resolveOutput() string {
	if b.opts.Output != "" {
		return b.opts.Output
	}
	if b.opts.Prefix == "" {
		return ""
	}

	name := filepath.Base(b.opts.Prefix)
	if b.opts.GOOS == "windows" {
		name += exeSuffix
	}
	return filepath.Join(b.opts.Prefix, "bin", name)
}

func (b *Builder) zigCmd(compiler, target string) string {
	bin := filepath.Join(b.zigPath, "zig")
	if runtime.GOOS == "windows" {
		bin += exeSuffix
	}
	return fmt.Sprintf("%s %s -target %s", bin, compiler, target)
}

func joinPrefixed(prefix string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = prefix + item
	}
	return strings.Join(parts, " ")
}

func appendPrefixed(dst []string, prefix string, items []string) []string {
	for _, item := range items {
		dst = append(dst, prefix+item)
	}
	return dst
}
