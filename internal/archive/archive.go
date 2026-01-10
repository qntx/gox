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
	defaultPerm     = 0o755
	maxSymlinkDepth = 10
)

var ErrPathTraversal = errors.New("path traversal detected")

// Format represents an archive format.
type Format int

const (
	FormatTarGz Format = iota
	FormatTarXz
	FormatZip
)

func (f Format) Ext() string {
	return [...]string{".tar.gz", ".tar.xz", ".zip"}[f]
}

// DetectFormat determines archive format from filename or URL.
func DetectFormat(name string) Format {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return FormatZip
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return FormatTarXz
	default:
		return FormatTarGz
	}
}

// FormatForOS returns the preferred archive format for a target OS.
func FormatForOS(goos string) Format {
	if goos == "windows" {
		return FormatZip
	}
	return FormatTarGz
}

// Extract extracts an archive to destDir, auto-stripping top-level directory.
func Extract(archivePath, destDir string) error {
	switch DetectFormat(archivePath) {
	case FormatZip:
		return extractZip(archivePath, destDir)
	case FormatTarXz:
		return extractTar(archivePath, destDir, newXzReader)
	default:
		return extractTar(archivePath, destDir, newGzipReader)
	}
}

func newGzipReader(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
func newXzReader(r io.Reader) (io.Reader, error)   { return xz.NewReader(r) }

// Download fetches URL, extracts to destDir, shows progress on stderr.
func Download(ctx context.Context, url, destDir string) error {
	return DownloadWithProgress(ctx, url, destDir, os.Stderr)
}

// DownloadWithProgress downloads and extracts with optional progress output.
func DownloadWithProgress(ctx context.Context, url, destDir string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "gox-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "archive"+DetectFormat(url).Ext())
	if err := downloadToFile(tmpFile, resp.Body, resp.ContentLength, w); err != nil {
		return err
	}

	if w != nil {
		fmt.Fprintln(w, "\nextracting...")
	}

	if err := os.MkdirAll(filepath.Dir(destDir), defaultPerm); err != nil {
		return err
	}
	return Extract(tmpFile, destDir)
}

// Create creates an archive from src for the given OS/arch.
func Create(src, goos, goarch string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}

	format := FormatForOS(goos)
	dest := filepath.Join(
		filepath.Dir(src),
		fmt.Sprintf("%s-%s-%s%s", filepath.Base(src), goos, goarch, format.Ext()),
	)

	if format == FormatZip {
		err = createZip(src, dest, info.IsDir())
	} else {
		err = createTarGz(src, dest, info.IsDir())
	}
	if err != nil {
		return "", err
	}
	return dest, nil
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	strip := zipStripPrefix(r.File)
	for _, f := range r.File {
		if err := extractZipEntry(f, destDir, strip); err != nil {
			return err
		}
	}
	return nil
}

func zipStripPrefix(files []*zip.File) string {
	if len(files) == 0 {
		return ""
	}
	return strings.SplitN(files[0].Name, "/", 2)[0] + "/"
}

func extractZipEntry(f *zip.File, destDir, strip string) error {
	name := strings.TrimPrefix(f.Name, strip)
	if name == "" {
		return nil
	}

	path, err := safePath(destDir, name)
	if err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(path, defaultPerm)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeFile(path, rc, f.Mode())
}

func extractTar(archivePath, destDir string, decompress func(io.Reader) (io.Reader, error)) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	dr, err := decompress(f)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	strip, err := tarStripPrefix(tar.NewReader(dr))
	if err != nil {
		return err
	}

	// Rewind for second pass
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dr, err = decompress(f)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	return processTar(tar.NewReader(dr), destDir, strip)
}

func tarStripPrefix(tr *tar.Reader) (string, error) {
	hdr, err := tr.Next()
	if err == io.EOF {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.SplitN(hdr.Name, "/", 2)[0] + "/", nil
}

type pendingSymlink struct {
	linkname, path string
}

func processTar(tr *tar.Reader, destDir, strip string) error {
	var symlinks []pendingSymlink

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

		path, err := safePath(destDir, name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, defaultPerm); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(path, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := symlink(hdr.Linkname, path); err != nil {
				// Windows: defer symlink resolution
				symlinks = append(symlinks, pendingSymlink{hdr.Linkname, path})
			}
		}
	}
	return resolveSymlinks(symlinks)
}

func symlink(target, path string) error {
	_ = os.Remove(path)
	return os.Symlink(target, path)
}

func resolveSymlinks(symlinks []pendingSymlink) error {
	if len(symlinks) == 0 {
		return nil
	}

	// Build path -> linkname map for chain resolution
	linkMap := make(map[string]string, len(symlinks))
	for _, sl := range symlinks {
		linkMap[sl.path] = sl.linkname
	}

	for _, sl := range symlinks {
		target := resolveChain(sl.path, sl.linkname, linkMap)
		// Skip if target doesn't exist (external dependency)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if err := copyFile(target, sl.path); err != nil {
			continue // Non-fatal: skip broken symlinks
		}
	}
	return nil
}

func resolveChain(base, linkname string, linkMap map[string]string) string {
	target := filepath.Join(filepath.Dir(base), linkname)
	for i := 0; i < maxSymlinkDepth; i++ {
		next, ok := linkMap[target]
		if !ok {
			return target
		}
		target = filepath.Join(filepath.Dir(target), next)
	}
	return target
}

func createTarGz(src, dest string, isDir bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if isDir {
		return walkTar(tw, src)
	}
	return addTarFile(tw, src, filepath.Base(src))
}

func walkTar(tw *tar.Writer, root string) error {
	baseDir := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
			header.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			return copyToWriter(tw, path)
		}
		return nil
	})
}

func addTarFile(tw *tar.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = name

	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	return copyToWriter(tw, src)
}

func createZip(src, dest string, isDir bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	if isDir {
		return walkZip(zw, src)
	}
	return addZipFile(zw, src, filepath.Base(src))
}

func walkZip(zw *zip.Writer, root string) error {
	baseDir := filepath.Dir(root)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			_, err := zw.Create(relPath + "/")
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		return copyToWriter(w, path)
	})
}

func addZipFile(zw *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	return copyToWriter(w, src)
}

func safePath(destDir, name string) (string, error) {
	path := filepath.Join(destDir, name)
	if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	return path, nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), defaultPerm); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	return writeFile(dst, in, info.Mode())
}

func copyToWriter(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func downloadToFile(path string, r io.Reader, total int64, w io.Writer) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	src := io.Reader(r)
	if w != nil && total > 0 {
		src = &progressReader{r: r, total: total, w: w}
	}

	_, err = io.Copy(f, src)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

type progressReader struct {
	r       io.Reader
	w       io.Writer
	total   int64
	current int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.current += int64(n)
	fmt.Fprintf(p.w, "\rdownloading: %.1f%%", float64(p.current)/float64(p.total)*100)
	return n, err
}
