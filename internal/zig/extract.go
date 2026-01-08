package zig

import (
	"archive/tar"
	"archive/zip"
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
			os.MkdirAll(path, 0755)
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
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader

	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gr.Close()
		reader = gr
	} else {
		xr, err := xz.NewReader(f)
		if err != nil {
			return err
		}
		reader = xr
	}

	tr := tar.NewReader(reader)

	var stripPrefix string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		parts := strings.SplitN(hdr.Name, "/", 2)
		if len(parts) > 0 {
			stripPrefix = parts[0]
			break
		}
	}

	f.Seek(0, io.SeekStart)
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		gr, _ := gzip.NewReader(f)
		defer gr.Close()
		reader = gr
	} else {
		xr, _ := xz.NewReader(f)
		reader = xr
	}
	tr = tar.NewReader(reader)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
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
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			os.Remove(path)
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		}
	}
	return nil
}
