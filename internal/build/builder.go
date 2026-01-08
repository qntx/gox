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

type Builder struct {
	zigPath string
	opts    *Options
}

func New(zigPath string, opts *Options) *Builder {
	return &Builder{zigPath: zigPath, opts: opts}
}

func (b *Builder) Run(ctx context.Context, packages []string) error {
	env := b.buildEnv()
	args := b.buildArgs(packages)

	if b.opts.Verbose {
		fmt.Fprintf(os.Stderr, "env: %v\n", env)
		fmt.Fprintf(os.Stderr, "go build %s\n", strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
	flags := make([]string, len(b.opts.IncludeDirs))
	for i, dir := range b.opts.IncludeDirs {
		flags[i] = "-I" + dir
	}
	return strings.Join(flags, " ")
}

func (b *Builder) cgoLdflags() string {
	var flags []string
	for _, dir := range b.opts.LibDirs {
		flags = append(flags, "-L"+dir)
	}
	for _, lib := range b.opts.Libs {
		flags = append(flags, "-l"+lib)
	}
	if b.opts.LinkMode == "static" {
		flags = append(flags, "-static")
	}
	return strings.Join(flags, " ")
}

func (b *Builder) buildArgs(packages []string) []string {
	args := []string{"build"}

	if b.opts.Output != "" {
		args = append(args, "-o", b.opts.Output)
	}

	// Build ldflags
	var ldflags []string

	// macOS cross-compile: disable DWARF to avoid dsymutil requirement
	if b.opts.GOOS == "darwin" && runtime.GOOS != "darwin" {
		ldflags = append(ldflags, "-w")
	}

	switch b.opts.LinkMode {
	case "static":
		ldflags = append(ldflags, "-linkmode=external", `-extldflags "-static"`)
	case "dynamic":
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

func (b *Builder) zigCC(target string) string {
	return b.zigCmd("cc", target)
}

func (b *Builder) zigCXX(target string) string {
	return b.zigCmd("c++", target)
}

func (b *Builder) zigCmd(compiler, target string) string {
	zigBin := filepath.Join(b.zigPath, "zig")
	if runtime.GOOS == "windows" {
		zigBin += ".exe"
	}
	return fmt.Sprintf("%s %s -target %s", zigBin, compiler, target)
}
