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

	"github.com/qntx/gox/internal/archive"
)

const pkgCacheDir = "pkg"

var githubReleaseRE = regexp.MustCompile(`^([^/]+)/([^@]+)@([^/]+)/(.+)$`)

// Package represents a downloadable dependency archive containing headers and/or libraries.
type Package struct {
	Source     string
	URL        string
	CacheDir   string
	IncludeDir string
	LibDir     string
}

// ParsePackage parses a package source string.
// Supported formats:
//   - Direct URL: https://example.com/archive.tar.gz
//   - GitHub release: owner/repo@version/asset.tar.gz
func ParsePackage(source string) (*Package, error) {
	pkg := &Package{Source: source}

	switch {
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		pkg.URL = source
		pkg.CacheDir = urlCacheKey(source)

	case githubReleaseRE.MatchString(source):
		m := githubReleaseRE.FindStringSubmatch(source)
		owner, repo, version, asset := m[1], m[2], m[3], m[4]
		pkg.URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
			owner, repo, version, asset)
		pkg.CacheDir = fmt.Sprintf("%s-%s-%s-%s", owner, repo, version, stripArchiveExt(asset))

	default:
		return nil, fmt.Errorf("invalid package source: %s (expected URL or owner/repo@version/asset)", source)
	}

	return pkg, nil
}

// Ensure downloads and extracts the package if not already cached.
func (p *Package) Ensure(ctx context.Context) error {
	root := pkgCacheRoot()
	pkgDir := filepath.Join(root, p.CacheDir)
	p.IncludeDir = filepath.Join(pkgDir, "include")
	p.LibDir = filepath.Join(pkgDir, "lib")

	if dirExists(pkgDir) {
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
	hasInclude := dirExists(p.IncludeDir)
	hasLib := dirExists(p.LibDir)

	if !hasInclude && !hasLib {
		return fmt.Errorf("package %s: missing both include/ and lib/ directories", p.Source)
	}
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pkgCacheRoot() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "gox", pkgCacheDir)
	}
	return filepath.Join(os.TempDir(), "gox", pkgCacheDir)
}

func urlCacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	name := filepath.Base(url)
	if idx := strings.LastIndex(name, "?"); idx > 0 {
		name = name[:idx]
	}
	return fmt.Sprintf("url-%s-%s", hex.EncodeToString(h[:8]), stripArchiveExt(name))
}

var archiveExtensions = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".zip"}

func stripArchiveExt(name string) string {
	for _, ext := range archiveExtensions {
		if strings.HasSuffix(name, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// EnsurePackages parses and downloads all package sources.
func EnsurePackages(ctx context.Context, sources []string) ([]*Package, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	packages := make([]*Package, 0, len(sources))
	for _, src := range sources {
		pkg, err := ParsePackage(src)
		if err != nil {
			return nil, err
		}
		if err := pkg.Ensure(ctx); err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// CollectPackagePaths returns existing include and lib directories from packages.
func CollectPackagePaths(packages []*Package) (includeDirs, libDirs []string) {
	for _, pkg := range packages {
		if dirExists(pkg.IncludeDir) {
			includeDirs = append(includeDirs, pkg.IncludeDir)
		}
		if dirExists(pkg.LibDir) {
			libDirs = append(libDirs, pkg.LibDir)
		}
	}
	return
}
