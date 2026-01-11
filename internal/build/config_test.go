package build

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/path/gox.toml")
		if err != ErrConfigNotFound {
			t.Errorf("LoadConfig() error = %v, want ErrConfigNotFound", err)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "gox.toml")
		content := `
[default]
zig-version = "0.15.0"
strip = true
verbose = false

[[target]]
name = "linux-amd64"
os = "linux"
arch = "amd64"
prefix = "./dist/linux"

[[target]]
name = "windows-amd64"
os = "windows"
arch = "amd64"
pack = true
`
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if cfg.Default.ZigVersion != "0.15.0" {
			t.Errorf("ZigVersion = %q, want 0.15.0", cfg.Default.ZigVersion)
		}
		if !cfg.Default.Strip {
			t.Error("Strip = false, want true")
		}
		if len(cfg.Targets) != 2 {
			t.Fatalf("len(Targets) = %d, want 2", len(cfg.Targets))
		}
		if cfg.Targets[0].Name != "linux-amd64" {
			t.Errorf("Targets[0].Name = %q, want linux-amd64", cfg.Targets[0].Name)
		}
		if cfg.Targets[1].Pack != true {
			t.Error("Targets[1].Pack = false, want true")
		}
	})

	t.Run("invalid toml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "gox.toml")
		if err := os.WriteFile(path, []byte("invalid[toml"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadConfig(path)
		if err == nil {
			t.Error("LoadConfig() should return error for invalid TOML")
		}
	})
}

func TestConfig_ToOptions(t *testing.T) {
	cfg := &Config{
		Default: ConfigDefault{
			ZigVersion: "0.15.0",
			Include:    []string{"/usr/include"},
			Strip:      true,
		},
		Targets: []ConfigTarget{
			{
				Name:    "linux-amd64",
				OS:      "linux",
				Arch:    "amd64",
				Prefix:  "./dist",
				Include: []string{"/opt/include"},
			},
			{
				Name:       "windows-amd64",
				OS:         "windows",
				Arch:       "amd64",
				ZigVersion: "0.14.0",
				Pack:       true,
			},
		},
	}

	t.Run("all targets", func(t *testing.T) {
		opts, err := cfg.ToOptions(nil)
		if err != nil {
			t.Fatalf("ToOptions() error = %v", err)
		}
		if len(opts) != 2 {
			t.Fatalf("len(opts) = %d, want 2", len(opts))
		}

		// First target
		if opts[0].GOOS != "linux" || opts[0].GOARCH != "amd64" {
			t.Errorf("opts[0] = %s/%s, want linux/amd64", opts[0].GOOS, opts[0].GOARCH)
		}
		if opts[0].ZigVersion != "0.15.0" {
			t.Errorf("opts[0].ZigVersion = %q, want 0.15.0", opts[0].ZigVersion)
		}
		if len(opts[0].IncludeDirs) != 2 {
			t.Errorf("len(opts[0].IncludeDirs) = %d, want 2", len(opts[0].IncludeDirs))
		}
		if !opts[0].Strip {
			t.Error("opts[0].Strip = false, want true")
		}

		// Second target with override
		if opts[1].ZigVersion != "0.14.0" {
			t.Errorf("opts[1].ZigVersion = %q, want 0.14.0", opts[1].ZigVersion)
		}
		if !opts[1].Pack {
			t.Error("opts[1].Pack = false, want true")
		}
	})

	t.Run("specific target", func(t *testing.T) {
		opts, err := cfg.ToOptions([]string{"windows-amd64"})
		if err != nil {
			t.Fatalf("ToOptions() error = %v", err)
		}
		if len(opts) != 1 {
			t.Fatalf("len(opts) = %d, want 1", len(opts))
		}
		if opts[0].GOOS != "windows" {
			t.Errorf("GOOS = %q, want windows", opts[0].GOOS)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		_, err := cfg.ToOptions([]string{"nonexistent"})
		if err == nil {
			t.Error("ToOptions() should return error for nonexistent target")
		}
	})

	t.Run("no targets defined", func(t *testing.T) {
		emptyCfg := &Config{
			Default: ConfigDefault{ZigVersion: "0.15.0"},
		}
		opts, err := emptyCfg.ToOptions(nil)
		if err != nil {
			t.Fatalf("ToOptions() error = %v", err)
		}
		if len(opts) != 1 {
			t.Fatalf("len(opts) = %d, want 1", len(opts))
		}
		if opts[0].ZigVersion != "0.15.0" {
			t.Errorf("ZigVersion = %q, want 0.15.0", opts[0].ZigVersion)
		}
	})
}

func TestMergeSlices(t *testing.T) {
	tests := []struct {
		name     string
		base     []string
		override []string
		want     int
	}{
		{"both empty", nil, nil, 0},
		{"base only", []string{"a", "b"}, nil, 2},
		{"override only", nil, []string{"c"}, 1},
		{"both", []string{"a"}, []string{"b", "c"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeSlices(tt.base, tt.override)
			if len(got) != tt.want {
				t.Errorf("len(mergeSlices()) = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFindConfig(t *testing.T) {
	// Create nested directory structure
	root := t.TempDir()
	subdir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write config at root
	configPath := filepath.Join(root, ConfigFile)
	if err := os.WriteFile(configPath, []byte("[default]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save and restore working directory
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	// Test finding config from subdirectory
	if err := os.Chdir(subdir); err != nil {
		t.Fatal(err)
	}

	found := findConfig()
	if found != configPath {
		t.Errorf("findConfig() = %q, want %q", found, configPath)
	}
}
