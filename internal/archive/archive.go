package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

const (
	perm         = 0o755
	maxLinkDepth = 10
)

var ErrPathTraversal = errors.New("path traversal")

// Format represents an archive format.
type Format int

const (
	TarGz Format = iota
	TarXz
	Zip
)

func (f Format) Ext() string {
	return [...]string{".tar.gz", ".tar.xz", ".zip"}[f]
}

// Detect determines format from filename.
func Detect(name string) Format {
	s := strings.ToLower(name)
	switch {
	case strings.HasSuffix(s, ".zip"):
		return Zip
	case strings.HasSuffix(s, ".tar.xz"), strings.HasSuffix(s, ".txz"):
		return TarXz
	default:
		return TarGz
	}
}

// ForOS returns preferred format for OS.
func ForOS(goos string) Format {
	if goos == "windows" {
		return Zip
	}
	return TarGz
}

// Extract extracts archive to destDir, stripping top-level directory.
func Extract(src, dst string) error {
	switch Detect(src) {
	case Zip:
		return unzip(src, dst)
	case TarXz:
		return untar(src, dst, xzReader)
	default:
		return untar(src, dst, gzReader)
	}
}

// Download fetches URL and extracts to dst.
func Download(ctx context.Context, url, dst string) error {
	return DownloadTo(ctx, url, dst, nil)
}

// DownloadTo downloads with optional progress tracking.
// If proxyReader is provided, it wraps the response body to track progress.
func DownloadTo(ctx context.Context, url, dst string, proxyReader func(io.Reader) io.Reader) error {
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
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.MkdirTemp("", "gox-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Wrap body with progress reader if provided
	body := io.Reader(resp.Body)
	if proxyReader != nil {
		body = proxyReader(body)
	}

	file := filepath.Join(tmp, "archive"+Detect(url).Ext())
	if err := fetchToReader(file, body); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), perm); err != nil {
		return err
	}
	return Extract(file, dst)
}

// ContentLength fetches the content length of a URL without downloading.
func ContentLength(ctx context.Context, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.ContentLength, nil
}

// Create creates archive from src for OS/arch.
func Create(src, goos, goarch string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}

	f := ForOS(goos)
	dst := filepath.Join(
		filepath.Dir(src),
		fmt.Sprintf("%s-%s-%s%s", filepath.Base(src), goos, goarch, f.Ext()),
	)

	if f == Zip {
		err = mkzip(src, dst, info.IsDir())
	} else {
		err = mktgz(src, dst, info.IsDir())
	}
	return dst, err
}

func gzReader(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
func xzReader(r io.Reader) (io.Reader, error) { return xz.NewReader(r) }

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	strip := zipPrefix(r.File)
	for _, f := range r.File {
		if err := unzipEntry(f, dst, strip); err != nil {
			return err
		}
	}
	return nil
}

func zipPrefix(files []*zip.File) string {
	if len(files) == 0 {
		return ""
	}
	// Get first directory component from first file
	first := strings.SplitN(files[0].Name, "/", 2)[0]
	if first == "" {
		return ""
	}
	prefix := first + "/"

	// Only strip if ALL files share this common root directory
	for _, f := range files {
		if !strings.HasPrefix(f.Name, prefix) {
			return "" // Multiple top-level dirs, don't strip
		}
	}
	return prefix
}

func unzipEntry(f *zip.File, dst, strip string) error {
	name := strings.TrimPrefix(f.Name, strip)
	if name == "" {
		return nil
	}

	p, err := safe(dst, name)
	if err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(p, perm)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return write(p, rc, f.Mode())
}

func untar(src, dst string, decomp func(io.Reader) (io.Reader, error)) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	dr, err := decomp(f)
	if err != nil {
		return err
	}

	// Single-pass extraction: detect prefix while extracting
	return untarSinglePass(tar.NewReader(dr), dst)
}

type link struct{ target, path string }

type bufferedEntry struct {
	hdr  tar.Header
	data []byte // nil for directories/symlinks
}

// untarSinglePass extracts tar in one pass, detecting common prefix on-the-fly.
// Buffers first few small entries to detect prefix, then streams the rest.
func untarSinglePass(tr *tar.Reader, dst string) error {
	var (
		prefix    string
		confirmed bool
		links     []link
		buffered  []bufferedEntry
		dirCache  = make(map[string]struct{}, 64) // Cache created directories
	)

	const (
		maxBufferEntries = 5
		maxBufferSize    = 1 << 20 // 1MB max per buffered file
	)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Phase 1: Buffer first few entries to detect prefix
		if !confirmed {
			dir := strings.SplitN(hdr.Name, "/", 2)[0]
			if dir != "" {
				if prefix == "" {
					prefix = dir + "/"
				} else if !strings.HasPrefix(hdr.Name, prefix) {
					// Multiple top-level dirs - flush without stripping
					prefix = ""
					confirmed = true
					for _, b := range buffered {
						if err := extractBuffered(&b, dst, "", &links, dirCache); err != nil {
							return err
						}
					}
					buffered = nil
				}
			}

			// Buffer small entries, stream large ones
			if !confirmed && hdr.Size <= maxBufferSize {
				entry := bufferedEntry{hdr: *hdr}
				if hdr.Typeflag == tar.TypeReg {
					entry.data, err = io.ReadAll(tr)
					if err != nil {
						return err
					}
				}
				buffered = append(buffered, entry)

				if len(buffered) >= maxBufferEntries {
					// Confirm prefix and flush buffer
					confirmed = true
					for _, b := range buffered {
						if err := extractBuffered(&b, dst, prefix, &links, dirCache); err != nil {
							return err
						}
					}
					buffered = nil
				}
				continue
			}

			// Large file encountered - flush buffer and confirm
			confirmed = true
			for _, b := range buffered {
				if err := extractBuffered(&b, dst, prefix, &links, dirCache); err != nil {
					return err
				}
			}
			buffered = nil
		}

		// Phase 2: Stream extract directly
		if err := streamExtract(tr, hdr, dst, prefix, &links, dirCache); err != nil {
			return err
		}
	}

	// Flush remaining buffered entries
	for _, b := range buffered {
		if err := extractBuffered(&b, dst, prefix, &links, dirCache); err != nil {
			return err
		}
	}

	return resolveLinks(links)
}

func extractBuffered(entry *bufferedEntry, dst, strip string, links *[]link, dirCache map[string]struct{}) error {
	name := strings.TrimPrefix(entry.hdr.Name, strip)
	if name == "" {
		return nil
	}

	p, err := safe(dst, name)
	if err != nil {
		return err
	}

	switch entry.hdr.Typeflag {
	case tar.TypeDir:
		dirCache[p] = struct{}{}
		return os.MkdirAll(p, perm)
	case tar.TypeReg:
		if err := mkdirCached(filepath.Dir(p), dirCache); err != nil {
			return err
		}
		return os.WriteFile(p, entry.data, os.FileMode(entry.hdr.Mode))
	case tar.TypeSymlink:
		if err := mklink(entry.hdr.Linkname, p); err != nil {
			*links = append(*links, link{entry.hdr.Linkname, p})
		}
	}
	return nil
}

// streamExtract writes file directly to disk without buffering in memory.
func streamExtract(tr *tar.Reader, hdr *tar.Header, dst, strip string, links *[]link, dirCache map[string]struct{}) error {
	name := strings.TrimPrefix(hdr.Name, strip)
	if name == "" {
		return nil
	}

	p, err := safe(dst, name)
	if err != nil {
		return err
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		dirCache[p] = struct{}{}
		return os.MkdirAll(p, perm)

	case tar.TypeReg:
		if err := mkdirCached(filepath.Dir(p), dirCache); err != nil {
			return err
		}
		return streamToFile(tr, p, os.FileMode(hdr.Mode))

	case tar.TypeSymlink:
		if err := mklink(hdr.Linkname, p); err != nil {
			*links = append(*links, link{hdr.Linkname, p})
		}
	}
	return nil
}

// streamToFile streams data directly to file with buffered I/O.
func streamToFile(r io.Reader, path string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	// Use 256KB buffer for optimal disk I/O
	buf := make([]byte, 256*1024)
	_, err = io.CopyBuffer(f, r, buf)
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}

// mkdirCached creates directory only if not already cached, reducing syscalls.
func mkdirCached(dir string, cache map[string]struct{}) error {
	if _, ok := cache[dir]; ok {
		return nil
	}
	if err := os.MkdirAll(dir, perm); err != nil {
		return err
	}
	cache[dir] = struct{}{}
	return nil
}

func mklink(target, path string) error {
	_ = os.Remove(path)
	return os.Symlink(target, path)
}

func resolveLinks(links []link) error {
	if len(links) == 0 {
		return nil
	}

	m := make(map[string]string, len(links))
	for _, l := range links {
		m[l.path] = l.target
	}

	for _, l := range links {
		t := resolve(l.path, l.target, m)
		if _, err := os.Stat(t); err != nil {
			continue
		}
		_ = cp(t, l.path)
	}
	return nil
}

func resolve(base, name string, m map[string]string) string {
	t := filepath.Join(filepath.Dir(base), name)
	for range maxLinkDepth {
		next, ok := m[t]
		if !ok {
			return t
		}
		t = filepath.Join(filepath.Dir(t), next)
	}
	return t
}

func mktgz(src, dst string, isDir bool) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if isDir {
		return tarWalk(tw, src)
	}
	return tarAdd(tw, src, filepath.Base(src))
}

func tarWalk(tw *tar.Writer, root string) error {
	base := filepath.Dir(root)
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(base, p)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)

		if info.IsDir() {
			hdr.Name += "/"
		} else if info.Mode()&os.ModeSymlink != 0 {
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			hdr.Linkname = l
			hdr.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return copyTo(tw, p)
		}
		return nil
	})
}

func tarAdd(tw *tar.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	return copyTo(tw, src)
}

func mkzip(src, dst string, isDir bool) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	if isDir {
		return zipWalk(zw, src)
	}
	return zipAdd(zw, src, filepath.Base(src))
}

func zipWalk(zw *zip.Writer, root string) error {
	base := filepath.Dir(root)
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(base, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		if info.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}

		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = rel
		hdr.Method = zip.Deflate

		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		return copyTo(w, p)
	})
}

func zipAdd(zw *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	return copyTo(w, src)
}

func safe(dst, name string) (string, error) {
	p := filepath.Join(dst, name)
	if !strings.HasPrefix(p, filepath.Clean(dst)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	return p, nil
}

func write(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), perm); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.CopyBuffer(f, r, make([]byte, 256*1024))
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}

func cp(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	return write(dst, in, info.Mode())
}

func copyTo(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.CopyBuffer(w, f, make([]byte, 256*1024))
	return err
}

func fetchToReader(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = io.CopyBuffer(f, r, make([]byte, 256*1024))
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}
