package build

import (
	"errors"
	"runtime"
)

var (
	goarchToZig = map[string]string{
		"amd64":   "x86_64",
		"386":     "x86",
		"arm64":   "aarch64",
		"arm":     "arm",
		"riscv64": "riscv64",
		"loong64": "loongarch64",
		"ppc64le": "powerpc64le",
		"s390x":   "s390x",
	}
	goosToZig = map[string]string{
		"linux":   "linux-gnu",
		"darwin":  "macos",
		"windows": "windows-gnu",
		"freebsd": "freebsd",
		"netbsd":  "netbsd",
		"android": "linux-android",
	}
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
	switch o.LinkMode {
	case "static", "dynamic", "auto":
		return nil
	default:
		return errors.New("linkmode must be: static, dynamic, or auto")
	}
}

func (o *Options) ZigTarget() string {
	arch := goarchToZig[o.GOARCH]
	if arch == "" {
		arch = o.GOARCH
	}
	osTarget := goosToZig[o.GOOS]
	if osTarget == "" {
		osTarget = o.GOOS
	}

	// Handle Linux ARM variants
	if o.GOOS == "linux" && o.GOARCH == "arm" {
		if o.LinkMode == "static" {
			osTarget = "linux-musleabihf"
		} else {
			osTarget = "linux-gnueabihf"
		}
	} else if o.LinkMode == "static" && o.GOOS == "linux" {
		osTarget = "linux-musl"
	}

	return arch + "-" + osTarget
}
