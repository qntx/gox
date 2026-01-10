package build

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

// LinkMode specifies how to link the binary.
type LinkMode string

const (
	LinkModeAuto    LinkMode = "auto"
	LinkModeStatic  LinkMode = "static"
	LinkModeDynamic LinkMode = "dynamic"
)

var validLinkModes = map[LinkMode]bool{
	LinkModeAuto: true, LinkModeStatic: true, LinkModeDynamic: true,
}

func (m LinkMode) Valid() bool { return validLinkModes[m] }

func (m LinkMode) IsStatic() bool { return m == LinkModeStatic }

// Zig target mappings
var (
	zigArch = map[string]string{
		"amd64": "x86_64", "386": "x86", "arm64": "aarch64",
		"arm": "arm", "riscv64": "riscv64", "loong64": "loongarch64",
		"ppc64le": "powerpc64le", "s390x": "s390x",
	}
	zigOS = map[string]string{
		"linux": "linux-gnu", "darwin": "macos", "windows": "windows-gnu",
		"freebsd": "freebsd", "netbsd": "netbsd",
	}
)

// Options configures a build operation.
type Options struct {
	GOOS        string
	GOARCH      string
	Output      string
	Prefix      string
	ZigVersion  string
	IncludeDirs []string
	LibDirs     []string
	Libs        []string
	Packages    []string
	BuildFlags  []string
	LinkMode    LinkMode
	NoRpath     bool
	Pack        bool
	Verbose     bool
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
		o.LinkMode = LinkModeAuto
	}
	if o.Prefix != "" {
		o.Prefix = filepath.Clean(o.Prefix)
	}
}

// Validate checks option constraints.
func (o *Options) Validate() error {
	if !o.LinkMode.Valid() {
		return fmt.Errorf("invalid linkmode %q: must be auto, static, or dynamic", o.LinkMode)
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

// ZigTarget returns the Zig cross-compilation target string.
func (o *Options) ZigTarget() string {
	return o.zigArch() + "-" + o.zigOSTarget()
}

func (o *Options) zigArch() string {
	if v, ok := zigArch[o.GOARCH]; ok {
		return v
	}
	return o.GOARCH
}

func (o *Options) zigOSTarget() string {
	if o.GOOS == "linux" {
		return o.linuxTarget()
	}
	if v, ok := zigOS[o.GOOS]; ok {
		return v
	}
	return o.GOOS
}

func (o *Options) linuxTarget() string {
	isArm := o.GOARCH == "arm"
	isStatic := o.LinkMode.IsStatic()

	switch {
	case isArm && isStatic:
		return "linux-musleabihf"
	case isArm:
		return "linux-gnueabihf"
	case isStatic:
		return "linux-musl"
	default:
		return "linux-gnu"
	}
}
