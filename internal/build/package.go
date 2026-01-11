package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/qntx/gox/internal/archive"
	"github.com/qntx/gox/internal/ui"
)

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Package represents a dependency archive with include/lib/bin directories.
type Package struct {
	Source  string
	URL     string
	Dir     string
	Include string
	Lib     string
	Bin     string
}

// CacheEntry represents a cached package with metadata.
type CacheEntry struct {
	Name         string
	Path         string
	Size         int64
	IncludeCount int
	LibCount     int
}

// ----------------------------------------------------------------------------
// Package Patterns
// ----------------------------------------------------------------------------

var (
	ghReleaseRE = regexp.MustCompile(`^([^/]+)/([^@]+)@([^/]+)/(.+)$`)
	archiveExts = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip"}
)

// ----------------------------------------------------------------------------
// Public Functions
// ----------------------------------------------------------------------------

// EnsureAll parses and downloads packages in parallel with progress.
func EnsureAll(ctx context.Context, sources []string) ([]*Package, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	pkgs := make([]*Package, len(sources))
	for i, s := range sources {
		p, err := parsePackage(s)
		if err != nil {
			return nil, err
		}
		p.resolvePaths()
		pkgs[i] = p
	}

	var toDownload []*Package
	for _, p := range pkgs {
		if !p.isCached() {
			toDownload = append(toDownload, p)
		}
	}
	if len(toDownload) == 0 {
		return pkgs, nil
	}

	sizes := make(map[string]int64)
	for _, p := range toDownload {
		if size, err := archive.ContentLength(ctx, p.URL); err == nil && size > 0 {
			sizes[p.URL] = size
		}
	}

	progress := ui.NewProgress()
	start := time.Now()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for _, p := range toDownload {
		bar := progress.AddBar(p.Dir, sizes[p.URL])
		wg.Go(func() {
			p.resolvePaths()
			if e := p.download(ctx, bar); e != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", p.Source, e))
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	progress.Wait()

	if len(errs) > 0 {
		ui.Error("Download failed: %v", errs[0])
		return nil, errs[0]
	}
	ui.Success("Downloaded %d package(s) in %s", len(toDownload), ui.FormatDuration(time.Since(start)))
	return pkgs, nil
}

// CollectPaths returns include, lib, and bin directories from packages.
func CollectPaths(pkgs []*Package) (inc, lib, bin []string) {
	for _, p := range pkgs {
		if isDir(p.Include) {
			inc = append(inc, p.Include)
		}
		if isDir(p.Lib) {
			lib = append(lib, resolveLibDir(p.Lib))
		}
		if isDir(p.Bin) {
			bin = append(bin, p.Bin)
		}
	}
	return
}

// ListCached returns all cached packages.
func ListCached() ([]CacheEntry, error) {
	root := cacheDir()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result []CacheEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name())
		result = append(result, CacheEntry{
			Name:         e.Name(),
			Path:         path,
			Size:         dirSize(path),
			IncludeCount: countFiles(filepath.Join(path, "include")),
			LibCount:     countFiles(filepath.Join(path, "lib")),
		})
	}
	return result, nil
}

// RemoveCached removes a cached package by name.
func RemoveCached(name string) error {
	return os.RemoveAll(filepath.Join(cacheDir(), name))
}

// RemoveAllCached removes all cached packages.
func RemoveAllCached() error {
	return os.RemoveAll(cacheDir())
}

// CacheDir returns the package cache directory path.
func CacheDir() string {
	return cacheDir()
}

// ----------------------------------------------------------------------------
// Package Methods
// ----------------------------------------------------------------------------

func (p *Package) resolvePaths() {
	dir := filepath.Join(cacheDir(), p.Dir)
	p.Include = filepath.Join(dir, "include")
	p.Lib = filepath.Join(dir, "lib")
	p.Bin = filepath.Join(dir, "bin")
}

func (p *Package) isCached() bool {
	return isDir(filepath.Join(cacheDir(), p.Dir))
}

func (p *Package) download(ctx context.Context, bar *ui.Bar) error {
	dir := filepath.Join(cacheDir(), p.Dir)

	var proxy func(io.Reader) io.Reader
	if bar != nil {
		proxy = bar.ProxyReader
	}

	if err := archive.DownloadTo(ctx, p.URL, dir, proxy); err != nil {
		os.RemoveAll(dir)
		if bar != nil {
			bar.Abort(true)
		}
		return err
	}
	if bar != nil {
		bar.Complete()
	}

	if !isDir(p.Include) && !isDir(p.Lib) {
		return fmt.Errorf("%s: missing include/ and lib/", p.Source)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func parsePackage(source string) (*Package, error) {
	p := &Package{Source: source}
	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		p.URL = source
		p.Dir = urlHash(source)
	case ghReleaseRE.MatchString(source):
		m := ghReleaseRE.FindStringSubmatch(source)
		p.URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", m[1], m[2], m[3], m[4])
		p.Dir = fmt.Sprintf("%s-%s-%s-%s", m[1], m[2], m[3], trimArchiveExt(m[4]))
	default:
		return nil, fmt.Errorf("invalid package: %s", source)
	}
	return p, nil
}

func resolveLibDir(libDir string) string {
	for _, arch := range []string{"x64", "x86_64", "amd64", "Win32", "x86"} {
		if sub := filepath.Join(libDir, arch); isDir(sub) {
			return sub
		}
	}
	return libDir
}

func cacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox", "pkg")
	}
	return filepath.Join(os.TempDir(), "gox", "pkg")
}

func urlHash(url string) string {
	h := sha256.Sum256([]byte(url))
	name := filepath.Base(url)
	if i := strings.LastIndex(name, "?"); i > 0 {
		name = name[:i]
	}
	return fmt.Sprintf("url-%s-%s", hex.EncodeToString(h[:8]), trimArchiveExt(name))
}

func trimArchiveExt(name string) string {
	for _, ext := range archiveExts {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func countFiles(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n
}
