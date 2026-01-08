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
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) > 0 {
			stripPrefix = parts[0]
			break
		}
	}

	for _, f := range r.File {
		name := strings.TrimPrefix(f.Name, stripPrefix+"/")
		if name == "" {
			continue
		}

		path := filepath.Join(destDir, name)

		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path: %s", path)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
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
	for {
		hdr, err := tr.Next()
		if err != nil {
			return ""
		}
		if parts := strings.SplitN(hdr.Name, "/", 2); len(parts) > 0 {
			return parts[0]
		}
	}
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

		path := filepath.Join(destDir, name)
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path: %s", path)
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
	_, err = io.Copy(out, r)
	out.Close()
	return err
}
