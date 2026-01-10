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

type Config struct {
	Default ConfigDefault  `toml:"default"`
	Targets []ConfigTarget `toml:"target"`
}

type ConfigDefault struct {
	ZigVersion string `toml:"zig-version"`
	Verbose    bool   `toml:"verbose"`
	Pack       bool   `toml:"pack"`
	LinkMode   string `toml:"linkmode"`
}

type ConfigTarget struct {
	Name       string   `toml:"name"`
	OS         string   `toml:"os"`
	Arch       string   `toml:"arch"`
	Output     string   `toml:"output"`
	Prefix     string   `toml:"prefix"`
	NoRpath    bool     `toml:"no-rpath"`
	Include    []string `toml:"include"`
	Lib        []string `toml:"lib"`
	Link       []string `toml:"link"`
	LinkMode   string   `toml:"linkmode"`
	Flags      []string `toml:"flags"`
	ZigVersion string   `toml:"zig-version"`
	Verbose    bool     `toml:"verbose"`
	Pack       bool     `toml:"pack"`
}

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

	for dir := cwd; ; {
		path := filepath.Join(dir, ConfigFile)
		if _, err := os.Stat(path); err == nil {
			return path
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func (c *Config) FindTarget(name string) (*ConfigTarget, error) {
	for i := range c.Targets {
		if c.Targets[i].Name == name {
			return &c.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("target %q not found", name)
}

func (c *Config) FindTargets(names []string) ([]*ConfigTarget, error) {
	if len(names) == 0 {
		targets := make([]*ConfigTarget, len(c.Targets))
		for i := range c.Targets {
			targets[i] = &c.Targets[i]
		}
		return targets, nil
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

func (c *Config) TargetNames() []string {
	names := make([]string, len(c.Targets))
	for i, t := range c.Targets {
		names[i] = t.Name
	}
	return names
}

func (c *Config) ToOptions(names []string) ([]*Options, error) {
	targets, err := c.FindTargets(names)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return []*Options{c.defaultOptions()}, nil
	}

	opts := make([]*Options, len(targets))
	for i, t := range targets {
		opts[i] = c.targetToOptions(t)
	}
	return opts, nil
}

func (c *Config) defaultOptions() *Options {
	return &Options{
		ZigVersion: c.Default.ZigVersion,
		Verbose:    c.Default.Verbose,
		Pack:       c.Default.Pack,
		LinkMode:   LinkMode(c.Default.LinkMode),
	}
}

func (c *Config) targetToOptions(t *ConfigTarget) *Options {
	opts := c.defaultOptions()
	opts.GOOS = t.OS
	opts.GOARCH = t.Arch
	opts.Output = t.Output
	opts.Prefix = t.Prefix
	opts.NoRpath = t.NoRpath
	opts.IncludeDirs = t.Include
	opts.LibDirs = t.Lib
	opts.Libs = t.Link
	opts.BuildFlags = t.Flags

	if t.ZigVersion != "" {
		opts.ZigVersion = t.ZigVersion
	}
	if t.LinkMode != "" {
		opts.LinkMode = LinkMode(t.LinkMode)
	}
	if t.Verbose {
		opts.Verbose = true
	}
	if t.Pack {
		opts.Pack = true
	}
	return opts
}
