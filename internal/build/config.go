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
	Default ConfigDefault  `toml:"default"`
	Targets []ConfigTarget `toml:"target"`
}

// ConfigDefault holds default values applied to all targets.
type ConfigDefault struct {
	ZigVersion string `toml:"zig-version"`
	LinkMode   string `toml:"linkmode"`
	Verbose    bool   `toml:"verbose"`
	Pack       bool   `toml:"pack"`
}

// ConfigTarget defines a build target configuration.
type ConfigTarget struct {
	Name       string   `toml:"name"`
	OS         string   `toml:"os"`
	Arch       string   `toml:"arch"`
	Output     string   `toml:"output"`
	Prefix     string   `toml:"prefix"`
	ZigVersion string   `toml:"zig-version"`
	LinkMode   string   `toml:"linkmode"`
	Include    []string `toml:"include"`
	Lib        []string `toml:"lib"`
	Link       []string `toml:"link"`
	Packages   []string `toml:"packages"`
	Flags      []string `toml:"flags"`
	NoRpath    bool     `toml:"no-rpath"`
	Verbose    bool     `toml:"verbose"`
	Pack       bool     `toml:"pack"`
}

// LoadConfig loads configuration from path, or searches upward from cwd if empty.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = findConfigFile()
		if path == "" {
			return nil, ErrConfigNotFound
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func findConfigFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for dir := cwd; ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		if parent := filepath.Dir(dir); parent == dir {
			return ""
		}
	}
}

// FindTarget returns the target with the given name.
func (c *Config) FindTarget(name string) (*ConfigTarget, error) {
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("target %q not found", name)
}

// FindTargets returns targets by names, or all targets if names is empty.
func (c *Config) FindTargets(names []string) ([]*ConfigTarget, error) {
	if len(names) == 0 {
		return c.allTargets(), nil
	}

	targets := make([]*ConfigTarget, 0, len(names))
	for _, name := range names {
		t, err := c.FindTarget(name)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func (c *Config) allTargets() []*ConfigTarget {
	targets := make([]*ConfigTarget, len(c.Targets))
	for i := range c.Targets {
		targets[i] = &c.Targets[i]
	}
	return targets
}

// TargetNames returns the names of all configured targets.
func (c *Config) TargetNames() []string {
	names := make([]string, len(c.Targets))
	for i, t := range c.Targets {
		names[i] = t.Name
	}
	return names
}

// ToOptions converts targets to build Options.
func (c *Config) ToOptions(names []string) ([]*Options, error) {
	targets, err := c.FindTargets(names)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return []*Options{c.baseOptions()}, nil
	}

	opts := make([]*Options, len(targets))
	for i, t := range targets {
		opts[i] = c.targetToOptions(t)
	}
	return opts, nil
}

func (c *Config) baseOptions() *Options {
	return &Options{
		ZigVersion: c.Default.ZigVersion,
		LinkMode:   LinkMode(c.Default.LinkMode),
		Verbose:    c.Default.Verbose,
		Pack:       c.Default.Pack,
	}
}

func (c *Config) targetToOptions(t *ConfigTarget) *Options {
	o := c.baseOptions()

	o.GOOS = t.OS
	o.GOARCH = t.Arch
	o.Output = t.Output
	o.Prefix = t.Prefix
	o.NoRpath = t.NoRpath
	o.IncludeDirs = t.Include
	o.LibDirs = t.Lib
	o.Libs = t.Link
	o.Packages = t.Packages
	o.BuildFlags = t.Flags

	// Target-level overrides
	if t.ZigVersion != "" {
		o.ZigVersion = t.ZigVersion
	}
	if t.LinkMode != "" {
		o.LinkMode = LinkMode(t.LinkMode)
	}

	// Boolean flags: target true overrides default
	o.Verbose = o.Verbose || t.Verbose
	o.Pack = o.Pack || t.Pack

	return o
}
