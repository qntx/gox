package zig

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

var errPathTraversal = errors.New("path traversal detected")

func extract(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destDir)
	}
	return extractTar(archivePath, destDir)
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	strip := ""
	if len(r.File) > 0 {
		strip = strings.SplitN(r.File[0].Name, "/", 2)[0] + "/"
	}

	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, strip)
		if name == "" {
			continue
		}

		path, err := safePath(destDir, name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, dirPerm); err != nil {
				return err
			}
			continue
		}

		if err := writeZipFile(f, path); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func extractTar(archivePath, destDir string) error {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}

	openReader := func() (io.Reader, error) {
		br := bytes.NewReader(data)
		if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
			return gzip.NewReader(br)
		}
		return xz.NewReader(br)
	}

	r, err := openReader()
	if err != nil {
		return err
	}

	strip := detectStripPrefix(tar.NewReader(r))

	r, err = openReader()
	if err != nil {
		return err
	}

	return processTar(tar.NewReader(r), destDir, strip)
}

func detectStripPrefix(tr *tar.Reader) string {
	hdr, err := tr.Next()
	if err != nil {
		return ""
	}
	return strings.SplitN(hdr.Name, "/", 2)[0] + "/"
}

func processTar(tr *tar.Reader, destDir, strip string) error {
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
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
			if err := os.MkdirAll(path, dirPerm); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(path, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			_ = os.Remove(path)
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		}
	}
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, r)
	return err
}

func safePath(destDir, name string) (string, error) {
	path := filepath.Join(destDir, name)
	if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %s", errPathTraversal, name)
	}
	return path, nil
}
