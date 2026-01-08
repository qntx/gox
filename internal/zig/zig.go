package zig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	indexURL  = "https://ziglang.org/download/index.json"
	cacheDir  = "gox"
	zigSubdir = "zig"
)

var (
	goarchToZigHost = map[string]string{
		"amd64": "x86_64",
		"386":   "x86",
		"arm64": "aarch64",
		"arm":   "armv7a",
	}
	goosToZigHost = map[string]string{
		"linux":   "linux",
		"darwin":  "macos",
		"windows": "windows",
	}
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

func (v *Version) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if ver, ok := raw["version"]; ok {
		if err := json.Unmarshal(ver, &v.Version); err != nil {
			return err
		}
	}
	if date, ok := raw["date"]; ok {
		if err := json.Unmarshal(date, &v.Date); err != nil {
			return err
		}
	}

	v.Tarball = make(map[string]Target)
	for key, val := range raw {
		if key == "version" || key == "date" || key == "notes" || key == "src" || key == "bootstrap" || key == "stdDocs" {
			continue
		}
		var t Target
		if err := json.Unmarshal(val, &t); err == nil && t.Tarball != "" {
			v.Tarball[key] = t
		}
	}
	return nil
}

func Ensure(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = "master"
	}
	cache := cacheRoot()

	zigDir := filepath.Join(cache, zigSubdir, version)
	zigBin := filepath.Join(zigDir, "zig")
	if runtime.GOOS == "windows" {
		zigBin += ".exe"
	}

	if _, err := os.Stat(zigBin); err == nil {
		return zigDir, nil
	}

	fmt.Fprintf(os.Stderr, "fetching zig version index...\n")

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

func cacheRoot() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, cacheDir)
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
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, err
	}
	return index, nil
}

func hostTarget() string {
	arch := goarchToZigHost[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	hostOS := goosToZigHost[runtime.GOOS]
	if hostOS == "" {
		hostOS = runtime.GOOS
	}
	return arch + "-" + hostOS
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

	ext := archiveExt(url)
	tmpFile := filepath.Join(tmpDir, "zig"+ext)

	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	reader := &progressReader{r: resp.Body, total: resp.ContentLength}

	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		return err
	}
	f.Close()

	fmt.Fprintf(os.Stderr, "\nextracting...\n")

	if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
		return err
	}

	return extract(tmpFile, destDir)
}

func archiveExt(url string) string {
	if strings.HasSuffix(url, ".zip") {
		return ".zip"
	}
	return ".tar.xz"
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
		pct := float64(p.read) / float64(p.total) * 100
		fmt.Fprintf(os.Stderr, "\rdownloading: %.1f%%", pct)
	}
	return n, err
}
