package build

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	LinkModeAuto    = "auto"
	LinkModeStatic  = "static"
	LinkModeDynamic = "dynamic"
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
	}
)

type Options struct {
	Output      string // Explicit output path (highest priority)
	Prefix      string // Installation prefix: creates <prefix>/bin/ and <prefix>/lib/
	NoRpath     bool   // Disable rpath when using prefix
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

func (o *Options) Normalize() {
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.LinkMode == "" {
		o.LinkMode = LinkModeAuto
	}
	if o.Prefix != "" {
		o.Prefix = filepath.Clean(o.Prefix)
	}
}

func (o *Options) Validate() error {
	switch o.LinkMode {
	case LinkModeStatic, LinkModeDynamic, LinkModeAuto:
	default:
		return fmt.Errorf("invalid linkmode %q: must be %s, %s, or %s",
			o.LinkMode, LinkModeStatic, LinkModeDynamic, LinkModeAuto)
	}

	if o.Output != "" && o.Prefix != "" {
		return errors.New("cannot specify both --output and --prefix")
	}

	if o.NoRpath && o.Prefix == "" {
		return errors.New("--no-rpath requires --prefix")
	}

	return nil
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
		if o.LinkMode == LinkModeStatic {
			osTarget = "linux-musleabihf"
		} else {
			osTarget = "linux-gnueabihf"
		}
	} else if o.LinkMode == LinkModeStatic && o.GOOS == "linux" {
		osTarget = "linux-musl"
	}

	return arch + "-" + osTarget
}
