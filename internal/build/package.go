package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/qntx/gox/internal/archive"
	"github.com/qntx/gox/internal/ui"
)

// Package represents a dependency archive.
type Package struct {
	Source  string
	URL     string
	Dir     string
	Include string
	Lib     string
}

var ghReleaseRE = regexp.MustCompile(`^([^/]+)/([^@]+)@([^/]+)/(.+)$`)

// Parse parses a package source (URL or owner/repo@version/asset).
func Parse(source string) (*Package, error) {
	p := &Package{Source: source}

	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		p.URL = source
		p.Dir = hashKey(source)
	case ghReleaseRE.MatchString(source):
		m := ghReleaseRE.FindStringSubmatch(source)
		p.URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", m[1], m[2], m[3], m[4])
		p.Dir = fmt.Sprintf("%s-%s-%s-%s", m[1], m[2], m[3], trimExt(m[4]))
	default:
		return nil, fmt.Errorf("invalid package: %s", source)
	}
	return p, nil
}

// Ensure downloads and extracts if not cached.
func (p *Package) Ensure(ctx context.Context) error {
	dir := filepath.Join(pkgCache(), p.Dir)
	p.Include = filepath.Join(dir, "include")
	p.Lib = filepath.Join(dir, "lib")

	if isDir(dir) {
		return p.check()
	}

	if err := archive.Download(ctx, p.URL, dir); err != nil {
		os.RemoveAll(dir)
		return err
	}
	return p.check()
}

func (p *Package) check() error {
	if !isDir(p.Include) && !isDir(p.Lib) {
		return fmt.Errorf("%s: missing include/ and lib/", p.Source)
	}
	return nil
}

// EnsureAll parses and downloads packages in parallel with progress.
func EnsureAll(ctx context.Context, sources []string) ([]*Package, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	pkgs := make([]*Package, len(sources))
	for i, s := range sources {
		p, err := Parse(s)
		if err != nil {
			return nil, err
		}
		pkgs[i] = p
	}

	// Check which packages need download
	var toDownload []*Package
	for _, p := range pkgs {
		dir := filepath.Join(pkgCache(), p.Dir)
		if !isDir(dir) {
			toDownload = append(toDownload, p)
		} else {
			p.Include = filepath.Join(dir, "include")
			p.Lib = filepath.Join(dir, "lib")
		}
	}

	if len(toDownload) == 0 {
		return pkgs, nil
	}

	// Download with progress tracking
	tracker := ui.NewTracker()
	ui.Downloading("", len(toDownload))

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		err error
	)
	for _, p := range toDownload {
		wg.Go(func() {
			if e := p.Ensure(ctx); e != nil {
				mu.Lock()
				if err == nil {
					err = e
				}
				mu.Unlock()
				return
			}
			tracker.Done(p.Dir, dirSize(filepath.Join(pkgCache(), p.Dir)))
		})
	}
	wg.Wait()

	if err != nil {
		ui.Error("Download failed: %v", err)
		return nil, err
	}
	ui.Success("Downloaded %d package(s) in %s", len(toDownload), ui.FormatDuration(tracker.Elapsed()))
	return pkgs, nil
}

// CollectPaths returns include and lib directories.
func CollectPaths(pkgs []*Package) (inc, lib []string) {
	for _, p := range pkgs {
		if isDir(p.Include) {
			inc = append(inc, p.Include)
		}
		if isDir(p.Lib) {
			lib = append(lib, p.Lib)
		}
	}
	return
}

// CachedPkg represents a cached package with metadata.
type CachedPkg struct {
	Name    string
	Path    string
	Size    int64
	Include int // file count
	Lib     int // file count
}

// ListCached returns all cached packages.
func ListCached() ([]CachedPkg, error) {
	root := pkgCache()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var pkgs []CachedPkg
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := CachedPkg{
			Name: e.Name(),
			Path: filepath.Join(root, e.Name()),
		}
		p.Size = dirSize(p.Path)
		p.Include = countFiles(filepath.Join(p.Path, "include"))
		p.Lib = countFiles(filepath.Join(p.Path, "lib"))
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// RemoveCached removes a cached package by name.
func RemoveCached(name string) error {
	return os.RemoveAll(filepath.Join(pkgCache(), name))
}

// RemoveAllCached removes all cached packages.
func RemoveAllCached() error {
	return os.RemoveAll(pkgCache())
}

// CacheDir returns the package cache directory path.
func CacheDir() string {
	return pkgCache()
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
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pkgCache() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox", "pkg")
	}
	return filepath.Join(os.TempDir(), "gox", "pkg")
}

func hashKey(url string) string {
	h := sha256.Sum256([]byte(url))
	name := filepath.Base(url)
	if i := strings.LastIndex(name, "?"); i > 0 {
		name = name[:i]
	}
	return fmt.Sprintf("url-%s-%s", hex.EncodeToString(h[:8]), trimExt(name))
}

var exts = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip"}

func trimExt(name string) string {
	for _, ext := range exts {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}
