package build

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents gox.toml structure.
type Config struct {
	Default ConfigDefault  `toml:"default"`
	Targets []ConfigTarget `toml:"target"`
}

// ConfigDefault holds values inherited by all targets.
type ConfigDefault struct {
	ZigVersion string   `toml:"zig-version"`
	LinkMode   string   `toml:"linkmode"`
	Include    []string `toml:"include"`
	Lib        []string `toml:"lib"`
	Link       []string `toml:"link"`
	Packages   []string `toml:"packages"`
	Flags      []string `toml:"flags"`
	Verbose    bool     `toml:"verbose"`
}

// ConfigTarget defines a platform-specific build configuration.
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
	Pack       bool     `toml:"pack"`
	Verbose    bool     `toml:"verbose"`
}

const ConfigFile = "gox.toml"

var ErrConfigNotFound = errors.New("config not found")

// LoadConfig loads config from path or searches upward from cwd.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = findConfig()
		if path == "" {
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

// ToOptions converts targets to Options slice.
func (c *Config) ToOptions(names []string) ([]*Options, error) {
	targets, err := c.selectTargets(names)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return []*Options{c.defaultOptions()}, nil
	}
	out := make([]*Options, len(targets))
	for i, t := range targets {
		out[i] = c.mergeOptions(t)
	}
	return out, nil
}

func (c *Config) selectTargets(names []string) ([]*ConfigTarget, error) {
	if len(names) == 0 {
		out := make([]*ConfigTarget, len(c.Targets))
		for i := range c.Targets {
			out[i] = &c.Targets[i]
		}
		return out, nil
	}
	out := make([]*ConfigTarget, 0, len(names))
	for _, name := range names {
		found := false
		for i := range c.Targets {
			if c.Targets[i].Name == name {
				out = append(out, &c.Targets[i])
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("target %q not found", name)
		}
	}
	return out, nil
}

func (c *Config) defaultOptions() *Options {
	d := &c.Default
	return &Options{
		ZigVersion:  d.ZigVersion,
		LinkMode:    LinkMode(d.LinkMode),
		IncludeDirs: append([]string(nil), d.Include...),
		LibDirs:     append([]string(nil), d.Lib...),
		Libs:        append([]string(nil), d.Link...),
		Packages:    append([]string(nil), d.Packages...),
		BuildFlags:  append([]string(nil), d.Flags...),
		Verbose:     d.Verbose,
	}
}

func (c *Config) mergeOptions(t *ConfigTarget) *Options {
	d := &c.Default
	zigVer, linkMode := t.ZigVersion, t.LinkMode
	if zigVer == "" {
		zigVer = d.ZigVersion
	}
	if linkMode == "" {
		linkMode = d.LinkMode
	}
	return &Options{
		GOOS:        t.OS,
		GOARCH:      t.Arch,
		Output:      t.Output,
		Prefix:      t.Prefix,
		ZigVersion:  zigVer,
		LinkMode:    LinkMode(linkMode),
		IncludeDirs: mergeSlices(d.Include, t.Include),
		LibDirs:     mergeSlices(d.Lib, t.Lib),
		Libs:        mergeSlices(d.Link, t.Link),
		Packages:    mergeSlices(d.Packages, t.Packages),
		BuildFlags:  mergeSlices(d.Flags, t.Flags),
		NoRpath:     t.NoRpath,
		Pack:        t.Pack,
		Verbose:     d.Verbose || t.Verbose,
	}
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
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func mergeSlices(base, override []string) []string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := make([]string, 0, len(base)+len(override))
	out = append(out, base...)
	out = append(out, override...)
	return out
}
