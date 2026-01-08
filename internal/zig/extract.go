package zig

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

func extract(archivePath, destDir string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, destDir)
	}
	return extractTarXz(archivePath, destDir)
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	var stripPrefix string
	if len(r.File) > 0 {
		stripPrefix = strings.SplitN(r.File[0].Name, "/", 2)[0]
	}

	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, stripPrefix+"/")
		if name == "" {
			continue
		}
		path, err := safePath(destDir, name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if err := extractZipFile(f, path); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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

func extractTarXz(archivePath, destDir string) error {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}

	newReader := func() (io.Reader, error) {
		buf := bytes.NewReader(data)
		if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
			return gzip.NewReader(buf)
		}
		return xz.NewReader(buf)
	}

	reader, err := newReader()
	if err != nil {
		return err
	}

	stripPrefix := findStripPrefix(tar.NewReader(reader))

	reader, err = newReader()
	if err != nil {
		return err
	}

	return extractTar(tar.NewReader(reader), destDir, stripPrefix)
}

func findStripPrefix(tr *tar.Reader) string {
	hdr, err := tr.Next()
	if err != nil {
		return ""
	}
	return strings.SplitN(hdr.Name, "/", 2)[0]
}

func extractTar(tr *tar.Reader, destDir, stripPrefix string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name := strings.TrimPrefix(hdr.Name, stripPrefix+"/")
		if name == "" {
			continue
		}

		path, err := safePath(destDir, name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := extractFile(path, tr, hdr.Mode); err != nil {
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

func extractFile(path string, r io.Reader, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func safePath(destDir, name string) (string, error) {
	path := filepath.Join(destDir, name)
	if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path: %s", path)
	}
	return path, nil
}
