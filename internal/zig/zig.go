package zig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	indexURL       = "https://ziglang.org/download/index.json"
	defaultVersion = "master"
	dirPerm        = 0755
)

var (
	archMap = map[string]string{"amd64": "x86_64", "386": "x86", "arm64": "aarch64", "arm": "armv7a"}
	osMap   = map[string]string{"linux": "linux", "darwin": "macos", "windows": "windows"}
)

type Index map[string]Version

type Version struct {
	Version string            `json:"version,omitempty"`
	Date    string            `json:"date,omitempty"`
	Tarball map[string]Target `json:"-"`
}

type Target struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

var skipKeys = map[string]bool{
	"version": true, "date": true, "notes": true,
	"src": true, "bootstrap": true, "stdDocs": true,
}

func (v *Version) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	_ = json.Unmarshal(raw["version"], &v.Version)
	_ = json.Unmarshal(raw["date"], &v.Date)

	v.Tarball = make(map[string]Target)
	for key, val := range raw {
		if skipKeys[key] {
			continue
		}
		var t Target
		if json.Unmarshal(val, &t) == nil && t.Tarball != "" {
			v.Tarball[key] = t
		}
	}
	return nil
}

func Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = defaultVersion
	}

	zigDir := Path(version)
	if binExists(zigDir) {
		return zigDir, nil
	}

	fmt.Fprintln(os.Stderr, "fetching zig version index...")

	index, err := fetchIndex(ctx)
	if err != nil {
		return "", err
	}

	ver, ok := index[version]
	if !ok {
		return "", fmt.Errorf("zig version %q not found", version)
	}

	host := hostTarget()
	target, ok := ver.Tarball[host]
	if !ok {
		return "", fmt.Errorf("no zig build for %s", host)
	}

	fmt.Fprintf(os.Stderr, "downloading zig %s for %s...\n", version, host)

	if err := download(ctx, target.Tarball, zigDir); err != nil {
		return "", err
	}
	return zigDir, nil
}

func binExists(dir string) bool {
	bin := filepath.Join(dir, "zig")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	_, err := os.Stat(bin)
	return err == nil
}

func cacheRoot() string {
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
		return nil, fmt.Errorf("fetch index: %s", resp.Status)
	}

	var index Index
	return index, json.NewDecoder(resp.Body).Decode(&index)
}

func hostTarget() string {
	arch := mapOr(archMap, runtime.GOARCH)
	os := mapOr(osMap, runtime.GOOS)
	return arch + "-" + os
}

func mapOr(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

func download(ctx context.Context, url, destDir string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}

	tmpDir, err := os.MkdirTemp("", "gox-zig-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	ext := ".tar.xz"
	if strings.HasSuffix(url, ".zip") {
		ext = ".zip"
	}

	tmpFile := filepath.Join(tmpDir, "zig"+ext)
	if err := downloadToFile(tmpFile, resp.Body, resp.ContentLength); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "\nextracting...")

	if err := os.MkdirAll(filepath.Dir(destDir), dirPerm); err != nil {
		return err
	}
	return extract(tmpFile, destDir)
}

func downloadToFile(path string, r io.Reader, total int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	pr := &progressReader{r: r, total: total}
	_, err = io.Copy(f, pr)

	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

type progressReader struct {
	r     io.Reader
	total int64
	read  int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 {
		fmt.Fprintf(os.Stderr, "\rdownloading: %.1f%%", float64(p.read)/float64(p.total)*100)
	}
	return n, err
}

func Path(version string) string {
	return filepath.Join(cacheRoot(), "zig", version)
}

func Installed() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(cacheRoot(), "zig"))
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

func Remove(version string) error {
	return os.RemoveAll(Path(version))
}
