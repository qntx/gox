package build

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

// ----------------------------------------------------------------------------
// LinkMode
// ----------------------------------------------------------------------------

// LinkMode specifies binary linking strategy.
type LinkMode string

const (
	LinkAuto    LinkMode = "auto"
	LinkStatic  LinkMode = "static"
	LinkDynamic LinkMode = "dynamic"
)

func (m LinkMode) Valid() bool {
	return m == LinkAuto || m == LinkStatic || m == LinkDynamic
}

func (m LinkMode) IsStatic() bool { return m == LinkStatic }

// ----------------------------------------------------------------------------
// Options
// ----------------------------------------------------------------------------

// Options configures a build operation.
type Options struct {
	// Platform
	GOOS   string
	GOARCH string

	// Output
	Output  string
	Prefix  string
	NoRpath bool
	Pack    bool

	// Toolchain
	ZigVersion string
	LinkMode   LinkMode

	// Dependencies
	IncludeDirs []string
	LibDirs     []string
	Libs        []string
	Packages    []string

	// Build
	BuildFlags []string
	Verbose    bool
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

// ----------------------------------------------------------------------------
// Zig Target
// ----------------------------------------------------------------------------

var (
	zigArch = map[string]string{
		"amd64":   "x86_64",
		"386":     "x86",
		"arm64":   "aarch64",
		"arm":     "arm",
		"riscv64": "riscv64",
		"loong64": "loongarch64",
		"ppc64le": "powerpc64le",
		"s390x":   "s390x",
	}
	zigOS = map[string]string{
		"linux":   "linux-gnu",
		"darwin":  "macos",
		"windows": "windows-gnu",
		"freebsd": "freebsd",
		"netbsd":  "netbsd",
	}
)

// ZigTarget returns the Zig cross-compilation target string.
func (o *Options) ZigTarget() string {
	return o.resolveArch() + "-" + o.resolveOS()
}

func (o *Options) resolveArch() string {
	if v, ok := zigArch[o.GOARCH]; ok {
		return v
	}
	return o.GOARCH
}

func (o *Options) resolveOS() string {
	if o.GOOS == "linux" {
		return o.linuxABI()
	}
	if v, ok := zigOS[o.GOOS]; ok {
		return v
	}
	return o.GOOS
}

func (o *Options) linuxABI() string {
	arm, static := o.GOARCH == "arm", o.LinkMode.IsStatic()
	switch {
	case arm && static:
		return "linux-musleabihf"
	case arm:
		return "linux-gnueabihf"
	case static:
		return "linux-musl"
	default:
		return "linux-gnu"
	}
}
