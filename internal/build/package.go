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

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Package represents a dependency archive.
type Package struct {
	Source  string
	URL     string
	Dir     string
	Include string
	Lib     string
}

// ----------------------------------------------------------------------------
// Public API
// ----------------------------------------------------------------------------

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

	fmt.Fprintf(os.Stderr, "pkg: %s\n", p.Source)
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

// EnsureAll parses and downloads packages in parallel.
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

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		err error
	)
	for _, p := range pkgs {
		wg.Go(func() {
			if e := p.Ensure(ctx); e != nil {
				mu.Lock()
				if err == nil {
					err = e
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if err != nil {
		return nil, err
	}
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

// ----------------------------------------------------------------------------
// Internal
// ----------------------------------------------------------------------------

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
