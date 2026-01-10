package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/qntx/gox/internal/pack"
)

const (
	dirPerm   = 0755
	exeSuffix = ".exe"
)

type Builder struct {
	zigPath string
	opts    *Options
}

func New(zigPath string, opts *Options) *Builder {
	return &Builder{zigPath: zigPath, opts: opts}
}

func (b *Builder) Run(ctx context.Context, packages []string) error {
	if err := b.ensureOutputDir(); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	env := b.buildEnv()
	args := b.buildArgs(packages)

	if b.opts.Verbose {
		b.printVerbose(env, args)
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	if !b.opts.Pack {
		return nil
	}
	return b.createArchive()
}

func (b *Builder) createArchive() error {
	src := cond(b.opts.Prefix != "", b.opts.Prefix, b.opts.Output)
	if src == "" {
		return fmt.Errorf("--pack requires --output or --prefix")
	}

	archive, err := pack.Archive(src, b.opts.GOOS, b.opts.GOARCH)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "archive: %s\n", archive)
	}
	return nil
}

func (b *Builder) printVerbose(env, args []string) {
	if output := b.resolveOutput(); output != "" {
		fmt.Fprintf(os.Stderr, "output: %s\n", output)
	}
	fmt.Fprintf(os.Stderr, "env: %v\ngo %s\n", env, strings.Join(args, " "))
}

func (b *Builder) ensureOutputDir() error {
	output := b.resolveOutput()
	if output == "" {
		return nil
	}

	if dir := filepath.Dir(output); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}

	if b.opts.Prefix != "" {
		return os.MkdirAll(filepath.Join(b.opts.Prefix, "lib"), dirPerm)
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
	if v := b.cgoCflags(); v != "" {
		env = append(env, "CGO_CFLAGS="+v)
	}
	if v := b.cgoLdflags(); v != "" {
		env = append(env, "CGO_LDFLAGS="+v)
	}
	return env
}

func (b *Builder) cgoCflags() string {
	return prefixJoin("-I", b.opts.IncludeDirs)
}

func (b *Builder) cgoLdflags() string {
	var flags []string
	flags = appendPrefixed(flags, "-L", b.opts.LibDirs)
	flags = appendPrefixed(flags, "-l", b.opts.Libs)

	if b.opts.LinkMode == LinkModeStatic {
		flags = append(flags, "-static")
	}
	if b.shouldAddRpath() {
		flags = append(flags, b.rpathFlag())
	}
	return strings.Join(flags, " ")
}

func (b *Builder) shouldAddRpath() bool {
	return b.opts.Prefix != "" && !b.opts.NoRpath && b.opts.LinkMode != LinkModeStatic
}

func (b *Builder) rpathFlag() string {
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

	exeName := filepath.Base(b.opts.Prefix)
	if b.opts.GOOS == "windows" {
		exeName += exeSuffix
	}
	return filepath.Join(b.opts.Prefix, "bin", exeName)
}

func (b *Builder) zigCmd(compiler, target string) string {
	zigBin := filepath.Join(b.zigPath, "zig")
	if runtime.GOOS == "windows" {
		zigBin += exeSuffix
	}
	return fmt.Sprintf("%s %s -target %s", zigBin, compiler, target)
}

func prefixJoin(prefix string, items []string) string {
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

func cond(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
