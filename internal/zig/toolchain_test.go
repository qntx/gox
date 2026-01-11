package zig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHostPlatform(t *testing.T) {
	platform := hostPlatform()

	// Should contain architecture
	archFound := false
	for _, arch := range []string{"x86_64", "aarch64", "x86", "armv7a"} {
		if strings.HasPrefix(platform, arch) {
			archFound = true
			break
		}
	}
	if !archFound {
		t.Errorf("hostPlatform() = %q, missing valid arch prefix", platform)
	}

	// Should contain OS
	osFound := false
	for _, osName := range []string{"linux", "macos", "windows"} {
		if strings.Contains(platform, osName) {
			osFound = true
			break
		}
	}
	if !osFound {
		t.Errorf("hostPlatform() = %q, missing valid OS", platform)
	}
}

func TestPath(t *testing.T) {
	path := Path("0.15.0")

	if !strings.Contains(path, "zig") {
		t.Errorf("Path() = %q, should contain 'zig'", path)
	}
	if !strings.Contains(path, "0.15.0") {
		t.Errorf("Path() = %q, should contain version", path)
	}
}

func TestIsInstalled(t *testing.T) {
	// Non-existent path
	if isInstalled("/nonexistent/path") {
		t.Error("isInstalled() = true for nonexistent path")
	}

	// Create fake zig installation
	dir := t.TempDir()
	zigBin := filepath.Join(dir, "zig")
	if runtime.GOOS == "windows" {
		zigBin += ".exe"
	}
	if err := os.WriteFile(zigBin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	if !isInstalled(dir) {
		t.Error("isInstalled() = false for valid installation")
	}
}

func TestBaseDir(t *testing.T) {
	dir := baseDir()

	if dir == "" {
		t.Error("baseDir() returned empty string")
	}
	if !strings.Contains(dir, "gox") {
		t.Errorf("baseDir() = %q, should contain 'gox'", dir)
	}
}

func TestRelease_UnmarshalJSON(t *testing.T) {
	data := `{
		"version": "0.15.0",
		"date": "2025-01-01",
		"x86_64-linux": {
			"tarball": "https://example.com/zig-0.15.0.tar.xz",
			"shasum": "abc123",
			"size": "12345678"
		},
		"aarch64-macos": {
			"tarball": "https://example.com/zig-0.15.0-macos.tar.xz",
			"shasum": "def456",
			"size": "12345678"
		},
		"src": {
			"tarball": "https://example.com/zig-src.tar.xz"
		},
		"notes": "release notes"
	}`

	var rel Release
	if err := json.Unmarshal([]byte(data), &rel); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if rel.Version != "0.15.0" {
		t.Errorf("Version = %q, want 0.15.0", rel.Version)
	}
	if rel.Date != "2025-01-01" {
		t.Errorf("Date = %q, want 2025-01-01", rel.Date)
	}

	// Should have 2 builds (excluding src, notes, etc.)
	if len(rel.Builds) != 2 {
		t.Errorf("len(Builds) = %d, want 2", len(rel.Builds))
	}

	linux, ok := rel.Builds["x86_64-linux"]
	if !ok {
		t.Fatal("missing x86_64-linux build")
	}
	if linux.Tarball != "https://example.com/zig-0.15.0.tar.xz" {
		t.Errorf("Tarball = %q", linux.Tarball)
	}
	if linux.Shasum != "abc123" {
		t.Errorf("Shasum = %q", linux.Shasum)
	}
}

func TestRelease_UnmarshalJSON_Empty(t *testing.T) {
	data := `{}`

	var rel Release
	if err := json.Unmarshal([]byte(data), &rel); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if rel.Version != "" {
		t.Errorf("Version = %q, want empty", rel.Version)
	}
	if len(rel.Builds) != 0 {
		t.Errorf("len(Builds) = %d, want 0", len(rel.Builds))
	}
}

func TestArchMap(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{"386", "x86"},
		{"amd64", "x86_64"},
		{"arm", "armv7a"},
		{"arm64", "aarch64"},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			if got := archMap[tt.goarch]; got != tt.want {
				t.Errorf("archMap[%q] = %q, want %q", tt.goarch, got, tt.want)
			}
		})
	}
}

func TestOSMap(t *testing.T) {
	if got := osMap["darwin"]; got != "macos" {
		t.Errorf("osMap[darwin] = %q, want macos", got)
	}
}

func TestSkipKeys(t *testing.T) {
	keys := []string{"bootstrap", "date", "notes", "src", "stdDocs", "version"}
	for _, k := range keys {
		if !skipKeys[k] {
			t.Errorf("skipKeys[%q] = false, want true", k)
		}
	}

	if skipKeys["x86_64-linux"] {
		t.Error("skipKeys[x86_64-linux] should be false")
	}
}
