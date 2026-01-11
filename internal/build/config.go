package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ----------------------------------------------------------------------------
// Constants & Errors
// ----------------------------------------------------------------------------

const ConfigFile = "gox.toml"

var ErrConfigNotFound = errors.New("config not found")

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Config represents gox.toml.
type Config struct {
	Default Default  `toml:"default"`
	Targets []Target `toml:"target"`
}

// Default holds inherited values for all targets.
type Default struct {
	ZigVersion string   `toml:"zig-version"`
	LinkMode   string   `toml:"linkmode"`
	Include    []string `toml:"include"`
	Lib        []string `toml:"lib"`
	Link       []string `toml:"link"`
	Packages   []string `toml:"packages"`
	Flags      []string `toml:"flags"`
	Verbose    bool     `toml:"verbose"`
}

// Target defines a build target.
type Target struct {
	Name       string   `toml:"name"`
	OS         string   `toml:"os"`
	Arch       string   `toml:"arch"`
	Output     string   `toml:"output"`
	Prefix     string   `toml:"prefix"`
	NoRpath    bool     `toml:"no-rpath"`
	Pack       bool     `toml:"pack"`
	ZigVersion string   `toml:"zig-version"`
	LinkMode   string   `toml:"linkmode"`
	Include    []string `toml:"include"`
	Lib        []string `toml:"lib"`
	Link       []string `toml:"link"`
	Packages   []string `toml:"packages"`
	Flags      []string `toml:"flags"`
	Verbose    bool     `toml:"verbose"`
}

// ----------------------------------------------------------------------------
// Loading
// ----------------------------------------------------------------------------

// LoadConfig loads config from path or searches upward from cwd.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		if path = findConfig(); path == "" {
			return nil, ErrConfigNotFound
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}

	var cfg Config
	return &cfg, toml.Unmarshal(data, &cfg)
}

func findConfig() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; ; {
		p := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		if parent := filepath.Dir(dir); parent == dir {
			return ""
		} else {
			dir = parent
		}
	}
}

// ----------------------------------------------------------------------------
// Conversion
// ----------------------------------------------------------------------------

// ToOptions converts targets to Options slice.
func (c *Config) ToOptions(names []string) ([]*Options, error) {
	targets, err := c.selectTargets(names)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return []*Options{c.baseOptions()}, nil
	}

	out := make([]*Options, len(targets))
	for i, t := range targets {
		out[i] = c.targetOptions(t)
	}
	return out, nil
}

func (c *Config) selectTargets(names []string) ([]*Target, error) {
	if len(names) == 0 {
		out := make([]*Target, len(c.Targets))
		for i := range c.Targets {
			out[i] = &c.Targets[i]
		}
		return out, nil
	}

	out := make([]*Target, 0, len(names))
	for _, name := range names {
		if t := c.findTarget(name); t != nil {
			out = append(out, t)
		} else {
			return nil, fmt.Errorf("target %q not found", name)
		}
	}
	return out, nil
}

func (c *Config) findTarget(name string) *Target {
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i]
		}
	}
	return nil
}

func (c *Config) baseOptions() *Options {
	d := &c.Default
	return &Options{
		ZigVersion:  d.ZigVersion,
		LinkMode:    LinkMode(d.LinkMode),
		IncludeDirs: d.Include,
		LibDirs:     d.Lib,
		Libs:        d.Link,
		Packages:    d.Packages,
		BuildFlags:  d.Flags,
		Verbose:     d.Verbose,
	}
}

func (c *Config) targetOptions(t *Target) *Options {
	d := &c.Default
	return &Options{
		GOOS:        t.OS,
		GOARCH:      t.Arch,
		Output:      t.Output,
		Prefix:      t.Prefix,
		NoRpath:     t.NoRpath,
		Pack:        t.Pack,
		ZigVersion:  coalesce(t.ZigVersion, d.ZigVersion),
		LinkMode:    LinkMode(coalesce(t.LinkMode, d.LinkMode)),
		IncludeDirs: append(d.Include, t.Include...),
		LibDirs:     append(d.Lib, t.Lib...),
		Libs:        append(d.Link, t.Link...),
		Packages:    append(d.Packages, t.Packages...),
		BuildFlags:  append(d.Flags, t.Flags...),
		Verbose:     d.Verbose || t.Verbose,
	}
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
