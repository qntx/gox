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

// DownloadTo downloads with optional progress writer.
// If pw is nil, no progress is reported.
func DownloadTo(ctx context.Context, url, dst string, pw io.Writer) error {
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

	file := filepath.Join(tmp, "archive"+Detect(url).Ext())
	if err := fetchTo(file, resp.Body, pw); err != nil {
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
	return strings.SplitN(files[0].Name, "/", 2)[0] + "/"
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

	strip, err := tarPrefix(tar.NewReader(dr))
	if err != nil {
		return err
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dr, err = decomp(f)
	if err != nil {
		return err
	}

	return untarAll(tar.NewReader(dr), dst, strip)
}

func tarPrefix(tr *tar.Reader) (string, error) {
	hdr, err := tr.Next()
	if err == io.EOF {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.SplitN(hdr.Name, "/", 2)[0] + "/", nil
}

type link struct{ target, path string }

func untarAll(tr *tar.Reader, dst, strip string) error {
	var links []link

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(hdr.Name, strip)
		if name == "" {
			continue
		}

		p, err := safe(dst, name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, perm); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := write(p, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := mklink(hdr.Linkname, p); err != nil {
				links = append(links, link{hdr.Linkname, p})
			}
		}
	}
	return resolveLinks(links)
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
	_, err = io.Copy(f, r)
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
	_, err = io.Copy(w, f)
	return err
}

func fetchTo(path string, r io.Reader, pw io.Writer) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	var dst io.Writer = f
	if pw != nil {
		dst = io.MultiWriter(f, pw)
	}

	_, err = io.Copy(dst, r)
	if e := f.Close(); err == nil {
		err = e
	}
	return err
}
