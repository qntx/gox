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
)

var githubReleaseRE = regexp.MustCompile(`^([^/]+)/([^@]+)@([^/]+)/(.+)$`)

// Package represents a dependency archive with headers and/or libraries.
type Package struct {
	Source     string
	URL        string
	CacheDir   string
	IncludeDir string
	LibDir     string
}

// ParsePackage parses a package source string.
// Formats: URL (https://...) or GitHub release (owner/repo@version/asset).
func ParsePackage(source string) (*Package, error) {
	pkg := &Package{Source: source}

	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		pkg.URL = source
		pkg.CacheDir = urlCacheKey(source)
	case githubReleaseRE.MatchString(source):
		m := githubReleaseRE.FindStringSubmatch(source)
		pkg.URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", m[1], m[2], m[3], m[4])
		pkg.CacheDir = fmt.Sprintf("%s-%s-%s-%s", m[1], m[2], m[3], stripExt(m[4]))
	default:
		return nil, fmt.Errorf("invalid package: %s", source)
	}
	return pkg, nil
}

// Ensure downloads and extracts the package if not cached.
func (p *Package) Ensure(ctx context.Context) error {
	pkgDir := filepath.Join(cacheRoot(), p.CacheDir)
	p.IncludeDir = filepath.Join(pkgDir, "include")
	p.LibDir = filepath.Join(pkgDir, "lib")

	if isDir(pkgDir) {
		return p.validate()
	}

	fmt.Fprintf(os.Stderr, "package: %s\n", p.Source)
	if err := archive.Download(ctx, p.URL, pkgDir); err != nil {
		os.RemoveAll(pkgDir)
		return fmt.Errorf("download: %w", err)
	}
	return p.validate()
}

func (p *Package) validate() error {
	if !isDir(p.IncludeDir) && !isDir(p.LibDir) {
		return fmt.Errorf("package %s: missing include/ and lib/", p.Source)
	}
	return nil
}

// EnsurePackages parses and downloads all packages in parallel.
func EnsurePackages(ctx context.Context, sources []string) ([]*Package, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	packages := make([]*Package, len(sources))
	for i, src := range sources {
		pkg, err := ParsePackage(src)
		if err != nil {
			return nil, err
		}
		packages[i] = pkg
	}

	// Parallel download using Go 1.25 wg.Go()
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	for _, pkg := range packages {
		wg.Go(func() {
			if err := pkg.Ensure(ctx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return packages, nil
}

// CollectPackagePaths returns include and lib directories from packages.
func CollectPackagePaths(packages []*Package) (inc, lib []string) {
	for _, p := range packages {
		if isDir(p.IncludeDir) {
			inc = append(inc, p.IncludeDir)
		}
		if isDir(p.LibDir) {
			lib = append(lib, p.LibDir)
		}
	}
	return
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func cacheRoot() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox", "pkg")
	}
	return filepath.Join(os.TempDir(), "gox", "pkg")
}

func urlCacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	name := filepath.Base(url)
	if i := strings.LastIndex(name, "?"); i > 0 {
		name = name[:i]
	}
	return fmt.Sprintf("url-%s-%s", hex.EncodeToString(h[:8]), stripExt(name))
}

var archiveExts = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip"}

func stripExt(name string) string {
	for _, ext := range archiveExts {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}
