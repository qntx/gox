package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const ConfigFile = "gox.toml"

var ErrConfigNotFound = errors.New("config file not found")

// Config represents the gox.toml configuration file.
type Config struct {
	Default Default  `toml:"default"`
	Targets []Target `toml:"target"`
}

// Default holds values inherited by all targets unless overridden.
// Suitable for: toolchain settings, common dependencies, global flags.
type Default struct {
	// Toolchain
	ZigVersion string `toml:"zig-version"`
	LinkMode   string `toml:"linkmode"`

	// Common dependencies (inherited, not replaced)
	Include  []string `toml:"include"`
	Lib      []string `toml:"lib"`
	Link     []string `toml:"link"`
	Packages []string `toml:"packages"`
	Flags    []string `toml:"flags"`

	// Behavior
	Verbose bool `toml:"verbose"`
}

// Target defines a build target configuration.
type Target struct {
	// Identity
	Name string `toml:"name"`
	OS   string `toml:"os"`
	Arch string `toml:"arch"`

	// Output
	Output  string `toml:"output"`
	Prefix  string `toml:"prefix"`
	NoRpath bool   `toml:"no-rpath"`
	Pack    bool   `toml:"pack"`

	// Toolchain (overrides default)
	ZigVersion string `toml:"zig-version"`
	LinkMode   string `toml:"linkmode"`

	// Dependencies (appended to default)
	Include  []string `toml:"include"`
	Lib      []string `toml:"lib"`
	Link     []string `toml:"link"`
	Packages []string `toml:"packages"`
	Flags    []string `toml:"flags"`

	// Behavior
	Verbose bool `toml:"verbose"`
}

// LoadConfig loads configuration from path, or searches upward from cwd.
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
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &cfg, nil
}

func findConfig() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := cwd; ; {
		path := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ToOptions converts selected targets to Options slice.
func (c *Config) ToOptions(names []string) ([]*Options, error) {
	targets, err := c.selectTargets(names)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return []*Options{c.defaultOptions()}, nil
	}

	opts := make([]*Options, len(targets))
	for i, t := range targets {
		opts[i] = c.toOptions(t)
	}
	return opts, nil
}

func (c *Config) selectTargets(names []string) ([]*Target, error) {
	if len(names) == 0 {
		targets := make([]*Target, len(c.Targets))
		for i := range c.Targets {
			targets[i] = &c.Targets[i]
		}
		return targets, nil
	}

	targets := make([]*Target, 0, len(names))
	for _, name := range names {
		found := false
		for i := range c.Targets {
			if c.Targets[i].Name == name {
				targets = append(targets, &c.Targets[i])
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("target %q not found", name)
		}
	}
	return targets, nil
}

func (c *Config) defaultOptions() *Options {
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

func (c *Config) toOptions(t *Target) *Options {
	d := &c.Default
	o := &Options{
		// Target identity
		GOOS:   t.OS,
		GOARCH: t.Arch,

		// Output
		Output:  t.Output,
		Prefix:  t.Prefix,
		NoRpath: t.NoRpath,
		Pack:    t.Pack,

		// Toolchain: target overrides default
		ZigVersion: or(t.ZigVersion, d.ZigVersion),
		LinkMode:   LinkMode(or(t.LinkMode, d.LinkMode)),

		// Dependencies: default + target (append)
		IncludeDirs: append(d.Include, t.Include...),
		LibDirs:     append(d.Lib, t.Lib...),
		Libs:        append(d.Link, t.Link...),
		Packages:    append(d.Packages, t.Packages...),
		BuildFlags:  append(d.Flags, t.Flags...),

		// Behavior: either true wins
		Verbose: d.Verbose || t.Verbose,
	}
	return o
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
