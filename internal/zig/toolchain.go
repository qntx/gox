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

const (
	indexURL = "https://ziglang.org/download/index.json"
	defVer   = "master"
)

// Index maps version names to releases.
type Index map[string]Release

// Release represents a Zig release.
type Release struct {
	Version string            `json:"version,omitempty"`
	Date    string            `json:"date,omitempty"`
	Targets map[string]Target `json:"-"`
}

// Target represents a downloadable build.
type Target struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

// Ensure downloads and caches a Zig version. Returns installation path.
func Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = defVer
	}

	dir := Path(version)
	if hasZig(dir) {
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

	host := platform()
	tgt, ok := rel.Targets[host]
	if !ok {
		return "", fmt.Errorf("no build for %s", host)
	}

	ui.Info("Downloading zig %s (%s)...", version, host)

	if err := archive.Download(ctx, tgt.Tarball, dir); err != nil {
		return "", err
	}

	ui.Success("Installed zig %s", version)
	return dir, nil
}

// Path returns the installation path for a version.
func Path(version string) string {
	return filepath.Join(cacheDir(), "zig", version)
}

// Installed returns all cached versions.
func Installed() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(cacheDir(), "zig"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Remove deletes a specific version.
func Remove(version string) error {
	return os.RemoveAll(Path(version))
}

// RemoveAll deletes all cached versions.
func RemoveAll() error {
	return os.RemoveAll(filepath.Join(cacheDir(), "zig"))
}

var (
	archMap = map[string]string{
		"amd64": "x86_64",
		"386":   "x86",
		"arm64": "aarch64",
		"arm":   "armv7a",
	}
	osMap = map[string]string{
		"darwin": "macos",
	}
	skipKeys = map[string]bool{
		"version": true, "date": true, "notes": true,
		"src": true, "bootstrap": true, "stdDocs": true,
	}
)

func (r *Release) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	_ = json.Unmarshal(raw["version"], &r.Version)
	_ = json.Unmarshal(raw["date"], &r.Date)

	r.Targets = make(map[string]Target)
	for k, v := range raw {
		if skipKeys[k] {
			continue
		}
		var t Target
		if json.Unmarshal(v, &t) == nil && t.Tarball != "" {
			r.Targets[k] = t
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

func platform() string {
	arch := archMap[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	goos := osMap[runtime.GOOS]
	if goos == "" {
		goos = runtime.GOOS
	}
	return arch + "-" + goos
}

func hasZig(dir string) bool {
	bin := filepath.Join(dir, "zig")
	if runtime.GOOS == "windows" {
		bin += ".exe"
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
