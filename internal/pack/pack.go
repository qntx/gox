package pack

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Archive(src, goos, goarch string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("stat source: %w", err)
	}

	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	dest := filepath.Join(filepath.Dir(src),
		fmt.Sprintf("%s-%s-%s%s", filepath.Base(src), goos, goarch, ext))

	if goos == "windows" {
		return dest, createZip(src, dest, info.IsDir())
	}
	return dest, createTarGz(src, dest, info.IsDir())
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

	if !isDir {
		return addTarFile(tw, src, filepath.Base(src))
	}

	baseDir := filepath.Dir(src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
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

		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(tw, path)
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
	return copyFile(tw, src)
}

func createZip(src, dest string, isDir bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	if !isDir {
		return addZipFile(zw, src, filepath.Base(src))
	}

	baseDir := filepath.Dir(src)
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
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
		return copyFile(w, path)
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
	return copyFile(w, src)
}

func copyFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
