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
	cc := b.zigCC(target)
	cxx := b.zigCXX(target)

	env := []string{
		"CGO_ENABLED=1",
		"GOOS=" + b.opts.GOOS,
		"GOARCH=" + b.opts.GOARCH,
		"CC=" + cc,
		"CXX=" + cxx,
	}

	if len(b.opts.IncludeDirs) > 0 {
		cflags := make([]string, len(b.opts.IncludeDirs))
		for i, dir := range b.opts.IncludeDirs {
			cflags[i] = "-I" + dir
		}
		env = append(env, "CGO_CFLAGS="+strings.Join(cflags, " "))
	}

	if len(b.opts.LibDirs) > 0 || len(b.opts.Libs) > 0 {
		var ldflags []string
		for _, dir := range b.opts.LibDirs {
			ldflags = append(ldflags, "-L"+dir)
		}
		for _, lib := range b.opts.Libs {
			ldflags = append(ldflags, "-l"+lib)
		}
		if b.opts.LinkMode == "static" {
			ldflags = append(ldflags, "-static")
		}
		env = append(env, "CGO_LDFLAGS="+strings.Join(ldflags, " "))
	}

	return env
}

func (b *Builder) buildArgs(packages []string) []string {
	args := []string{"build"}

	if b.opts.Output != "" {
		args = append(args, "-o", b.opts.Output)
	}

	switch b.opts.LinkMode {
	case "static":
		args = append(args, `-ldflags=-linkmode=external -extldflags "-static"`)
	case "dynamic":
		args = append(args, `-ldflags=-linkmode=external`)
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
