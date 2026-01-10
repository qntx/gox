package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	return cmd.Run()
}

func (b *Builder) printVerbose(env, args []string) {
	if output := b.resolveOutput(); output != "" {
		fmt.Fprintf(os.Stderr, "output: %s\n", output)
	}
	fmt.Fprintf(os.Stderr, "env: %v\n", env)
	fmt.Fprintf(os.Stderr, "go %s\n", strings.Join(args, " "))
}

func (b *Builder) ensureOutputDir() error {
	output := b.resolveOutput()
	if output == "" {
		return nil
	}

	dir := filepath.Dir(output)
	if dir == "" || dir == "." {
		return nil
	}

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}

	// Create lib directory when using prefix
	if b.opts.Prefix != "" {
		libDir := filepath.Join(b.opts.Prefix, "lib")
		if err := os.MkdirAll(libDir, dirPerm); err != nil {
			return err
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
		"CC=" + b.zigCC(target),
		"CXX=" + b.zigCXX(target),
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
	if len(b.opts.IncludeDirs) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, dir := range b.opts.IncludeDirs {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString("-I")
		sb.WriteString(dir)
	}
	return sb.String()
}

func (b *Builder) cgoLdflags() string {
	var sb strings.Builder
	for _, dir := range b.opts.LibDirs {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString("-L")
		sb.WriteString(dir)
	}
	for _, lib := range b.opts.Libs {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString("-l")
		sb.WriteString(lib)
	}
	if b.opts.LinkMode == LinkModeStatic {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString("-static")
	}
	// Add rpath when using prefix (unless disabled or static linking)
	if b.shouldAddRpath() {
		for _, flag := range b.rpathFlags() {
			if sb.Len() > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(flag)
		}
	}
	return sb.String()
}

func (b *Builder) shouldAddRpath() bool {
	return b.opts.Prefix != "" && !b.opts.NoRpath && b.opts.LinkMode != LinkModeStatic
}

func (b *Builder) rpathFlags() []string {
	switch b.opts.GOOS {
	case "linux", "freebsd", "netbsd":
		return []string{"-Wl,-rpath,$ORIGIN/../lib"}
	case "darwin":
		return []string{"-Wl,-rpath,@executable_path/../lib"}
	default:
		return nil
	}
}

func (b *Builder) buildArgs(packages []string) []string {
	args := []string{"build"}

	output := b.resolveOutput()
	if output != "" {
		args = append(args, "-o", output)
	}

	// Build ldflags
	var ldflags []string

	// macOS cross-compile: disable DWARF to avoid dsymutil requirement
	if b.opts.GOOS == "darwin" && runtime.GOOS != "darwin" {
		ldflags = append(ldflags, "-w")
	}

	switch b.opts.LinkMode {
	case LinkModeStatic:
		ldflags = append(ldflags, "-linkmode=external", `-extldflags "-static"`)
	case LinkModeDynamic:
		ldflags = append(ldflags, "-linkmode=external")
	}

	if len(ldflags) > 0 {
		args = append(args, "-ldflags="+strings.Join(ldflags, " "))
	}

	args = append(args, b.opts.BuildFlags...)

	if len(packages) > 0 {
		args = append(args, packages...)
	} else {
		args = append(args, ".")
	}

	return args
}

func (b *Builder) resolveOutput() string {
	// Explicit output takes highest priority
	if b.opts.Output != "" {
		return b.opts.Output
	}

	// No prefix means let Go decide output location
	if b.opts.Prefix == "" {
		return ""
	}

	// Use prefix basename as executable name
	exeName := filepath.Base(b.opts.Prefix)
	if b.opts.GOOS == "windows" {
		exeName += exeSuffix
	}

	return filepath.Join(b.opts.Prefix, "bin", exeName)
}

func (b *Builder) zigCC(target string) string {
	return b.zigCmd("cc", target)
}

func (b *Builder) zigCXX(target string) string {
	return b.zigCmd("c++", target)
}

func (b *Builder) zigCmd(compiler, target string) string {
	zigBin := filepath.Join(b.zigPath, "zig")
	if runtime.GOOS == "windows" {
		zigBin += exeSuffix
	}
	return fmt.Sprintf("%s %s -target %s", zigBin, compiler, target)
}
