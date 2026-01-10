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

// Constants
const (
	DirPerm         = 0o755
	maxSymlinkDepth = 10
)

// Errors
var ErrPathTraversal = errors.New("path traversal detected")

// Format represents an archive format.
type Format int

const (
	FormatTarGz Format = iota
	FormatTarXz
	FormatZip
)

// Ext returns the file extension for the format.
func (f Format) Ext() string {
	return [...]string{".tar.gz", ".tar.xz", ".zip"}[f]
}

// DetectFormat determines the archive format from a filename or URL.
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

// FormatForOS returns the appropriate archive format for a target OS.
func FormatForOS(goos string) Format {
	if goos == "windows" {
		return FormatZip
	}
	return FormatTarGz
}

// Extract extracts an archive to the destination directory.
// Supports .tar.gz, .tgz, .tar.xz, .txz, and .zip formats.
// Automatically strips the top-level directory from the archive.
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

type decompressorFunc func(io.Reader) (io.Reader, error)

func newGzipReader(r io.Reader) (io.Reader, error) { return gzip.NewReader(r) }
func newXzReader(r io.Reader) (io.Reader, error)   { return xz.NewReader(r) }

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	strip := detectZipStripPrefix(r.File)

	for _, f := range r.File {
		if err := extractZipEntry(f, destDir, strip); err != nil {
			return err
		}
	}
	return nil
}

func detectZipStripPrefix(files []*zip.File) string {
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
		return os.MkdirAll(path, DirPerm)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer rc.Close()

	return writeToFile(path, rc, f.Mode())
}

func extractTar(archivePath, destDir string, decompress decompressorFunc) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	dr, err := decompress(f)
	if err != nil {
		return fmt.Errorf("init decompressor: %w", err)
	}

	tr := tar.NewReader(dr)
	strip, err := detectTarStripPrefix(tr)
	if err != nil {
		return err
	}

	// Reopen and rewind for second pass
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	dr, err = decompress(f)
	if err != nil {
		return fmt.Errorf("reinit decompressor: %w", err)
	}

	return processTar(tar.NewReader(dr), destDir, strip)
}

func detectTarStripPrefix(tr *tar.Reader) (string, error) {
	hdr, err := tr.Next()
	if err == io.EOF {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read tar header: %w", err)
	}
	return strings.SplitN(hdr.Name, "/", 2)[0] + "/", nil
}

type deferredSymlink struct {
	target string
	path   string
}

func processTar(tr *tar.Reader, destDir, strip string) error {
	var symlinks []deferredSymlink

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
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
			if err := os.MkdirAll(path, DirPerm); err != nil {
				return fmt.Errorf("mkdir %s: %w", path, err)
			}
		case tar.TypeReg:
			if err := writeToFile(path, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := createSymlink(hdr.Linkname, path); err != nil {
				// Windows fallback: defer file copy
				symlinks = append(symlinks, deferredSymlink{
					target: filepath.Join(filepath.Dir(path), hdr.Linkname),
					path:   path,
				})
			}
		}
	}

	// Resolve deferred symlinks (Windows compatibility)
	for _, sl := range symlinks {
		if err := copyFileResolved(sl.target, sl.path); err != nil {
			return fmt.Errorf("symlink fallback %s: %w", sl.path, err)
		}
	}
	return nil
}

func createSymlink(target, path string) error {
	_ = os.Remove(path)
	return os.Symlink(target, path)
}

func writeToFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return fmt.Errorf("mkdir for %s: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()

	if copyErr != nil {
		return fmt.Errorf("write %s: %w", path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", path, closeErr)
	}
	return nil
}

func copyFileResolved(src, dst string) error {
	resolved, err := resolveSymlink(src)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(resolved)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	return writeToFile(dst, srcFile, info.Mode())
}

func resolveSymlink(path string) (string, error) {
	resolved := path
	for i := 0; i < maxSymlinkDepth; i++ {
		info, err := os.Lstat(resolved)
		if err != nil {
			return "", fmt.Errorf("lstat %s: %w", resolved, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return resolved, nil
		}

		link, err := os.Readlink(resolved)
		if err != nil {
			return "", fmt.Errorf("readlink %s: %w", resolved, err)
		}

		if filepath.IsAbs(link) {
			resolved = link
		} else {
			resolved = filepath.Join(filepath.Dir(resolved), link)
		}
	}
	return resolved, nil
}

func safePath(destDir, name string) (string, error) {
	path := filepath.Join(destDir, name)
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	if !strings.HasPrefix(path, cleanDest) {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, name)
	}
	return path, nil
}

func copyFromFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("copy %s: %w", path, err)
	}
	return nil
}

// Download downloads a URL to a temporary file, extracts it to destDir,
// and shows progress on stderr.
func Download(ctx context.Context, url, destDir string) error {
	return DownloadWithProgress(ctx, url, destDir, os.Stderr)
}

// DownloadWithProgress downloads and extracts with progress reported to w.
// Pass nil to disable progress output.
func DownloadWithProgress(ctx context.Context, url, destDir string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "gox-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	format := DetectFormat(url)
	tmpFile := filepath.Join(tmpDir, "archive"+format.Ext())

	if err := downloadToFile(tmpFile, resp.Body, resp.ContentLength, w); err != nil {
		return err
	}

	if w != nil {
		fmt.Fprintln(w, "\nextracting...")
	}

	if err := os.MkdirAll(filepath.Dir(destDir), DirPerm); err != nil {
		return fmt.Errorf("mkdir dest: %w", err)
	}
	return Extract(tmpFile, destDir)
}

func downloadToFile(path string, r io.Reader, total int64, progressW io.Writer) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	var src io.Reader = r
	if progressW != nil && total > 0 {
		src = &progressReader{r: r, total: total, w: progressW}
	}

	_, copyErr := io.Copy(f, src)
	closeErr := f.Close()

	if copyErr != nil {
		return fmt.Errorf("download: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close download: %w", closeErr)
	}
	return nil
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

// Create creates an archive from src (file or directory) for the given OS/arch.
// Returns the path to the created archive.
func Create(src, goos, goarch string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}

	format := FormatForOS(goos)
	dest := filepath.Join(
		filepath.Dir(src),
		fmt.Sprintf("%s-%s-%s%s", filepath.Base(src), goos, goarch, format.Ext()),
	)

	var createErr error
	if format == FormatZip {
		createErr = createZip(src, dest, info.IsDir())
	} else {
		createErr = createTarGz(src, dest, info.IsDir())
	}

	if createErr != nil {
		return "", createErr
	}
	return dest, nil
}

func createTarGz(src, dest string, isDir bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	if !isDir {
		return addFileToTar(tw, src, filepath.Base(src))
	}
	return walkTar(tw, src)
}

func walkTar(tw *tar.Writer, root string) error {
	baseDir := filepath.Dir(root)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("tar header: %w", err)
		}
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		} else if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", path, err)
			}
			header.Linkname = link
			header.Typeflag = tar.TypeSymlink
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}

		if info.Mode().IsRegular() {
			return copyFromFile(tw, path)
		}
		return nil
	})
}

func addFileToTar(tw *tar.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	header.Name = name

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}
	return copyFromFile(tw, src)
}

func createZip(src, dest string, isDir bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	if !isDir {
		return addFileToZip(zw, src, filepath.Base(src))
	}
	return walkZip(zw, src)
}

func walkZip(zw *zip.Writer, root string) error {
	baseDir := filepath.Dir(root)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			_, err := zw.Create(relPath + "/")
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("zip header: %w", err)
		}
		header.Name = relPath
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create zip entry: %w", err)
		}
		return copyFromFile(w, path)
	})
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("zip header: %w", err)
	}
	header.Name = name
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry: %w", err)
	}
	return copyFromFile(w, src)
}
