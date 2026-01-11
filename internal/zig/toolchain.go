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
	"github.com/qntx/gox/internal/ui"
)

// Index maps version names to releases.
type Index map[string]Release

// Release represents a Zig release with platform-specific builds.
type Release struct {
	Version string           `json:"version,omitempty"`
	Date    string           `json:"date,omitempty"`
	Builds  map[string]Build `json:"-"`
}

// Build represents a downloadable platform build.
type Build struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

const (
	indexURL       = "https://ziglang.org/download/index.json"
	defaultVersion = "master"
)

var (
	archMap = map[string]string{
		"386":   "x86",
		"amd64": "x86_64",
		"arm":   "armv7a",
		"arm64": "aarch64",
	}
	osMap = map[string]string{
		"darwin": "macos",
	}
	skipKeys = map[string]bool{
		"bootstrap": true,
		"date":      true,
		"notes":     true,
		"src":       true,
		"stdDocs":   true,
		"version":   true,
	}
)

// Ensure downloads and caches a Zig version. Returns installation path.
func Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = defaultVersion
	}

	dir := Path(version)
	if isInstalled(dir) {
		return dir, nil
	}

	idx, err := fetchIndex(ctx)
	if err != nil {
		return "", err
	}

	rel, ok := idx[version]
	if !ok {
		return "", fmt.Errorf("version %q not found", version)
	}

	platform := hostPlatform()
	build, ok := rel.Builds[platform]
	if !ok {
		return "", fmt.Errorf("no build for %s", platform)
	}

	size, _ := archive.ContentLength(ctx, build.Tarball)

	progress := ui.NewProgress()
	bar := progress.AddBar(fmt.Sprintf("zig %s (%s)", version, platform), size)

	if err := archive.DownloadTo(ctx, build.Tarball, dir, bar.ProxyReader); err != nil {
		bar.Abort(true)
		progress.Wait()
		return "", err
	}
	bar.Complete()
	progress.Wait()

	ui.Success("Installed zig %s", version)
	return dir, nil
}

// Path returns the installation path for a version.
func Path(version string) string {
	return filepath.Join(baseDir(), "zig", version)
}

// Installed returns all cached versions.
func Installed() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(baseDir(), "zig"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions, nil
}

// Remove deletes a specific version.
func Remove(version string) error {
	return os.RemoveAll(Path(version))
}

// RemoveAll deletes all cached versions.
func RemoveAll() error {
	return os.RemoveAll(filepath.Join(baseDir(), "zig"))
}

func (r *Release) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	_ = json.Unmarshal(raw["version"], &r.Version)
	_ = json.Unmarshal(raw["date"], &r.Date)

	r.Builds = make(map[string]Build)
	for k, v := range raw {
		if skipKeys[k] {
			continue
		}
		var b Build
		if json.Unmarshal(v, &b) == nil && b.Tarball != "" {
			r.Builds[k] = b
		}
	}
	return nil
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

	var idx Index
	return idx, json.NewDecoder(resp.Body).Decode(&idx)
}

func hostPlatform() string {
	arch := archMap[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	os := osMap[runtime.GOOS]
	if os == "" {
		os = runtime.GOOS
	}
	return arch + "-" + os
}

func isInstalled(dir string) bool {
	bin := filepath.Join(dir, "zig")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	_, err := os.Stat(bin)
	return err == nil
}

func baseDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox")
	}
	return filepath.Join(os.TempDir(), "gox")
}
