package build

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

// LinkMode specifies binary linking strategy.
type LinkMode string

// Options configures a build operation.
type Options struct {
	GOOS        string
	GOARCH      string
	Output      string
	Prefix      string
	ZigVersion  string
	LinkMode    LinkMode
	IncludeDirs []string
	LibDirs     []string
	BinDirs     []string
	Libs        []string
	Packages    []string
	BuildFlags  []string
	NoRpath     bool
	Pack        bool
	Verbose     bool
}

const (
	LinkAuto    LinkMode = "auto"
	LinkStatic  LinkMode = "static"
	LinkDynamic LinkMode = "dynamic"
)

var (
	zigArch = map[string]string{
		"386":     "x86",
		"amd64":   "x86_64",
		"arm":     "arm",
		"arm64":   "aarch64",
		"loong64": "loongarch64",
		"ppc64le": "powerpc64le",
		"riscv64": "riscv64",
		"s390x":   "s390x",
	}
	zigOS = map[string]string{
		"darwin":  "macos",
		"freebsd": "freebsd",
		"linux":   "linux-gnu",
		"netbsd":  "netbsd",
		"windows": "windows-gnu",
	}
)

func (m LinkMode) Valid() bool {
	return m == LinkAuto || m == LinkStatic || m == LinkDynamic
}

func (m LinkMode) IsStatic() bool {
	return m == LinkStatic
}

// Normalize applies defaults for unset fields.
func (o *Options) Normalize() {
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.LinkMode == "" {
		o.LinkMode = LinkAuto
	}
	if o.Prefix != "" {
		o.Prefix = filepath.Clean(o.Prefix)
	}
}

// Validate checks option constraints.
func (o *Options) Validate() error {
	if !o.LinkMode.Valid() {
		return fmt.Errorf("invalid linkmode: %q", o.LinkMode)
	}
	if o.Output != "" && o.Prefix != "" {
		return errors.New("--output and --prefix are mutually exclusive")
	}
	if o.NoRpath && o.Prefix == "" {
		return errors.New("--no-rpath requires --prefix")
	}
	if o.Pack && o.Output == "" && o.Prefix == "" {
		return errors.New("--pack requires --output or --prefix")
	}
	return nil
}

// ZigTarget returns the Zig cross-compilation target triple.
func (o *Options) ZigTarget() string {
	arch := zigArch[o.GOARCH]
	os := zigOS[o.GOOS]
	if o.GOOS == "linux" {
		os = o.linuxABI()
	}
	return arch + "-" + os
}

func (o *Options) linuxABI() string {
	if o.LinkMode.IsStatic() {
		if o.GOARCH == "arm" {
			return "linux-musleabihf"
		}
		return "linux-musl"
	}
	if o.GOARCH == "arm" {
		return "linux-gnueabihf"
	}
	return "linux-gnu"
}
