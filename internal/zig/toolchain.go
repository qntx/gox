package zig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/qntx/gox/internal/archive"
)

const (
	indexURL       = "https://ziglang.org/download/index.json"
	defaultVersion = "master"
	exeSuffix      = ".exe"
)

// Host platform mappings to Zig target names
var (
	hostArch = map[string]string{
		"amd64": "x86_64", "386": "x86", "arm64": "aarch64", "arm": "armv7a",
	}
	hostOS = map[string]string{
		"linux": "linux", "darwin": "macos", "windows": "windows",
	}
)

// Index represents the Zig download index.
type Index map[string]Release

// Release represents a Zig release version with available targets.
type Release struct {
	Version string            `json:"version,omitempty"`
	Date    string            `json:"date,omitempty"`
	Targets map[string]Target `json:"-"`
}

// Target represents a downloadable Zig build.
type Target struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

var metadataKeys = map[string]bool{
	"version": true, "date": true, "notes": true,
	"src": true, "bootstrap": true, "stdDocs": true,
}

func (r *Release) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	_ = json.Unmarshal(raw["version"], &r.Version)
	_ = json.Unmarshal(raw["date"], &r.Date)

	r.Targets = make(map[string]Target)
	for key, val := range raw {
		if metadataKeys[key] {
			continue
		}
		var t Target
		if json.Unmarshal(val, &t) == nil && t.Tarball != "" {
			r.Targets[key] = t
		}
	}
	return nil
}

// Ensure downloads and caches a Zig version if not already present.
// Returns the path to the Zig installation directory.
func Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = defaultVersion
	}

	dir := Path(version)
	if hasBinary(dir) {
		return dir, nil
	}

	fmt.Fprintln(os.Stderr, "fetching zig version index...")

	index, err := fetchIndex(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch index: %w", err)
	}

	release, ok := index[version]
	if !ok {
		return "", fmt.Errorf("zig version %q not found", version)
	}

	host := hostPlatform()
	target, ok := release.Targets[host]
	if !ok {
		return "", fmt.Errorf("no zig build for %s", host)
	}

	fmt.Fprintf(os.Stderr, "downloading zig %s for %s...\n", version, host)

	if err := archive.Download(ctx, target.Tarball, dir); err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	return dir, nil
}

// Path returns the cache path for a Zig version.
func Path(version string) string {
	return filepath.Join(cacheDir(), "zig", version)
}

// Installed returns all cached Zig versions.
func Installed() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(cacheDir(), "zig"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}

	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions, nil
}

// Remove deletes a specific Zig version from cache.
func Remove(version string) error {
	return os.RemoveAll(Path(version))
}

// RemoveAll deletes all cached Zig versions.
func RemoveAll() error {
	return os.RemoveAll(filepath.Join(cacheDir(), "zig"))
}

func hasBinary(dir string) bool {
	bin := filepath.Join(dir, "zig")
	if runtime.GOOS == "windows" {
		bin += exeSuffix
	}
	_, err := os.Stat(bin)
	return err == nil
}

func cacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox")
	}
	return filepath.Join(os.TempDir(), "gox")
}

func fetchIndex(ctx context.Context) (Index, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var index Index
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return index, nil
}

func hostPlatform() string {
	arch := hostArch[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	os := hostOS[runtime.GOOS]
	if os == "" {
		os = runtime.GOOS
	}
	return arch + "-" + os
}
