package build

import (
	"errors"
	"runtime"
)

type Options struct {
	Output      string
	GOOS        string
	GOARCH      string
	ZigVersion  string
	IncludeDirs []string
	LibDirs     []string
	Libs        []string
	LinkMode    string
	BuildFlags  []string
	Interactive bool
	Verbose     bool
}

func (o *Options) Validate() error {
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.LinkMode == "" {
		o.LinkMode = "auto"
	}
	if o.LinkMode != "static" && o.LinkMode != "dynamic" && o.LinkMode != "auto" {
		return errors.New("linkmode must be: static, dynamic, or auto")
	}
	return nil
}

func (o *Options) ZigTarget() string {
	return zigTarget(o.GOOS, o.GOARCH)
}

func zigTarget(goos, goarch string) string {
	arch := map[string]string{
		"amd64":   "x86_64",
		"386":     "x86",
		"arm64":   "aarch64",
		"arm":     "arm",
		"riscv64": "riscv64",
	}[goarch]
	if arch == "" {
		arch = goarch
	}

	os := map[string]string{
		"linux":   "linux-gnu",
		"darwin":  "macos",
		"windows": "windows-gnu",
		"freebsd": "freebsd",
	}[goos]
	if os == "" {
		os = goos
	}

	return arch + "-" + os
}
