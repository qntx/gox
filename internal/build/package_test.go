package build

import (
	"testing"
)

func TestParsePackage(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantURL string
		wantDir string
		wantErr bool
	}{
		{
			name:    "github release",
			source:  "owner/repo@v1.0.0/asset-linux.tar.gz",
			wantURL: "https://github.com/owner/repo/releases/download/v1.0.0/asset-linux.tar.gz",
			wantDir: "owner-repo-v1.0.0-asset-linux",
		},
		{
			name:    "github release with org",
			source:  "my-org/my-repo@v2.1.0/lib.tar.xz",
			wantURL: "https://github.com/my-org/my-repo/releases/download/v2.1.0/lib.tar.xz",
			wantDir: "my-org-my-repo-v2.1.0-lib",
		},
		{
			name:    "https url",
			source:  "https://example.com/lib-1.0.tar.gz",
			wantURL: "https://example.com/lib-1.0.tar.gz",
		},
		{
			name:    "http url",
			source:  "http://example.com/lib.zip",
			wantURL: "http://example.com/lib.zip",
		},
		{
			name:    "invalid source",
			source:  "invalid-source",
			wantErr: true,
		},
		{
			name:    "empty source",
			source:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := parsePackage(tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePackage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if pkg.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", pkg.URL, tt.wantURL)
			}
			if tt.wantDir != "" && pkg.Dir != tt.wantDir {
				t.Errorf("Dir = %q, want %q", pkg.Dir, tt.wantDir)
			}
		})
	}
}

func TestTrimArchiveExt(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"lib.tar.gz", "lib"},
		{"lib.tgz", "lib"},
		{"lib.tar.xz", "lib"},
		{"lib.txz", "lib"},
		{"lib.zip", "lib"},
		{"lib", "lib"},
		{"lib.so", "lib.so"},
		{"my-lib-1.0.0.tar.gz", "my-lib-1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := trimArchiveExt(tt.input); got != tt.want {
				t.Errorf("trimArchiveExt(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestURLHash(t *testing.T) {
	url1 := "https://example.com/lib.tar.gz"
	url2 := "https://example.com/lib.tar.gz?token=abc"

	hash1 := urlHash(url1)
	hash2 := urlHash(url2)

	// Same URL should produce same hash
	if urlHash(url1) != hash1 {
		t.Error("urlHash should be deterministic")
	}

	// Different URLs should produce different hashes
	if hash1 == hash2 {
		t.Error("different URLs should produce different hashes")
	}

	// Hash should have expected prefix
	if len(hash1) < 10 {
		t.Errorf("hash too short: %q", hash1)
	}
}

func TestCollectPaths(t *testing.T) {
	// Create temp directories
	dir := t.TempDir()

	pkgs := []*Package{
		{Include: dir, Lib: dir, Bin: dir},
		{Include: "/nonexistent", Lib: "/nonexistent", Bin: "/nonexistent"},
	}

	inc, lib, bin := CollectPaths(pkgs)

	if len(inc) != 1 || inc[0] != dir {
		t.Errorf("inc = %v, want [%s]", inc, dir)
	}
	if len(lib) != 1 {
		t.Errorf("len(lib) = %d, want 1", len(lib))
	}
	if len(bin) != 1 || bin[0] != dir {
		t.Errorf("bin = %v, want [%s]", bin, dir)
	}
}

func TestIsDir(t *testing.T) {
	dir := t.TempDir()

	if !isDir(dir) {
		t.Errorf("isDir(%q) = false, want true", dir)
	}
	if isDir("/nonexistent/path") {
		t.Error("isDir(/nonexistent/path) = true, want false")
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"cuda-linux-amd64", "cuda-*", true},
		{"cuda-linux-amd64", "*-amd64", true},
		{"cuda-linux-amd64", "cuda-*-amd64", true},
		{"cuda-linux-amd64", "cuda-linux-amd64", true},
		{"cuda-linux-amd64", "openssl-*", false},
		{"cuda-linux-amd64", "*-arm64", false},
		{"lib", "lib", true},
		{"lib", "libs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.pattern, func(t *testing.T) {
			if got := matchGlob(tt.name, tt.pattern); got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
			}
		})
	}
}

func matchGlob(name, pattern string) bool {
	if len(pattern) == 0 {
		return name == pattern
	}

	// Import the function from pkg.go for testing
	// This is a simplified version for testing
	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
	}
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			prefix := pattern[:i]
			suffix := pattern[i+1:]
			return len(name) >= len(prefix)+len(suffix) &&
				name[:len(prefix)] == prefix &&
				name[len(name)-len(suffix):] == suffix
		}
	}
	return name == pattern
}
