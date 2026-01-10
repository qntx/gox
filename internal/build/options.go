package build

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

type LinkMode string

const (
	LinkModeAuto    LinkMode = "auto"
	LinkModeStatic  LinkMode = "static"
	LinkModeDynamic LinkMode = "dynamic"
)

func (m LinkMode) Valid() bool {
	switch m {
	case LinkModeAuto, LinkModeStatic, LinkModeDynamic:
		return true
	}
	return false
}

var (
	archMap = map[string]string{
		"amd64": "x86_64", "386": "x86", "arm64": "aarch64",
		"arm": "arm", "riscv64": "riscv64", "loong64": "loongarch64",
		"ppc64le": "powerpc64le", "s390x": "s390x",
	}
	osMap = map[string]string{
		"linux": "linux-gnu", "darwin": "macos", "windows": "windows-gnu",
		"freebsd": "freebsd", "netbsd": "netbsd",
	}
)

type Options struct {
	Output      string
	Prefix      string
	NoRpath     bool
	Pack        bool
	GOOS        string
	GOARCH      string
	ZigVersion  string
	IncludeDirs []string
	LibDirs     []string
	Libs        []string
	LinkMode    LinkMode
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
	if !o.LinkMode.Valid() {
		return fmt.Errorf("invalid linkmode %q: must be static, dynamic, or auto", o.LinkMode)
	}
	if o.Output != "" && o.Prefix != "" {
		return errors.New("cannot specify both --output and --prefix")
	}
	if o.NoRpath && o.Prefix == "" {
		return errors.New("--no-rpath requires --prefix")
	}
	if o.Pack && o.Output == "" && o.Prefix == "" {
		return errors.New("--pack requires --output or --prefix")
	}
	return nil
}

func (o *Options) ZigTarget() string {
	arch := mapOr(archMap, o.GOARCH, o.GOARCH)
	osTarget := o.zigOS()
	return arch + "-" + osTarget
}

func (o *Options) zigOS() string {
	if o.GOOS == "linux" {
		return o.linuxTarget()
	}
	return mapOr(osMap, o.GOOS, o.GOOS)
}

func (o *Options) linuxTarget() string {
	if o.GOARCH == "arm" {
		if o.LinkMode == LinkModeStatic {
			return "linux-musleabihf"
		}
		return "linux-gnueabihf"
	}
	if o.LinkMode == LinkModeStatic {
		return "linux-musl"
	}
	return "linux-gnu"
}

func mapOr(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}
