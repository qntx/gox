package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		want Format
	}{
		{"file.tar.gz", TarGz},
		{"file.tgz", TarGz},
		{"file.TAR.GZ", TarGz},
		{"file.tar.xz", TarXz},
		{"file.txz", TarXz},
		{"file.zip", Zip},
		{"file.ZIP", Zip},
		{"file", TarGz},
		{"file.unknown", TarGz},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.name); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFormat_Ext(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{TarGz, ".tar.gz"},
		{TarXz, ".tar.xz"},
		{Zip, ".zip"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.format.Ext(); got != tt.want {
				t.Errorf("Format.Ext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForOS(t *testing.T) {
	tests := []struct {
		goos string
		want Format
	}{
		{"windows", Zip},
		{"linux", TarGz},
		{"darwin", TarGz},
		{"freebsd", TarGz},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := ForOS(tt.goos); got != tt.want {
				t.Errorf("ForOS(%q) = %v, want %v", tt.goos, got, tt.want)
			}
		})
	}
}

func TestSafe(t *testing.T) {
	dst := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", "subdir/file.txt", false},
		{"path traversal up", "../etc/passwd", true},
		{"path traversal nested", "subdir/../../etc/passwd", true},
		{"double dots in name", "file..txt", false},
		{"simple file", "file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := safe(dst, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("safe(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestExtract_TarGz(t *testing.T) {
	// Create test tar.gz
	srcDir := t.TempDir()
	tarPath := filepath.Join(srcDir, "test.tar.gz")
	createTestTarGz(t, tarPath, map[string]string{
		"root/file1.txt":        "content1",
		"root/subdir/file2.txt": "content2",
	})

	// Extract
	dstDir := t.TempDir()
	if err := Extract(tarPath, dstDir); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	// Verify - top-level "root" should be stripped
	assertFileContent(t, filepath.Join(dstDir, "file1.txt"), "content1")
	assertFileContent(t, filepath.Join(dstDir, "subdir", "file2.txt"), "content2")
}

func TestExtract_Zip(t *testing.T) {
	// Create test zip
	srcDir := t.TempDir()
	zipPath := filepath.Join(srcDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"root/file1.txt":        "content1",
		"root/subdir/file2.txt": "content2",
	})

	// Extract
	dstDir := t.TempDir()
	if err := Extract(zipPath, dstDir); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	// Verify - top-level "root" should be stripped
	assertFileContent(t, filepath.Join(dstDir, "file1.txt"), "content1")
	assertFileContent(t, filepath.Join(dstDir, "subdir", "file2.txt"), "content2")
}

func TestExtract_NoStrip(t *testing.T) {
	// Create tar.gz with multiple top-level directories
	srcDir := t.TempDir()
	tarPath := filepath.Join(srcDir, "test.tar.gz")
	createTestTarGz(t, tarPath, map[string]string{
		"dir1/file1.txt": "content1",
		"dir2/file2.txt": "content2",
	})

	// Extract
	dstDir := t.TempDir()
	if err := Extract(tarPath, dstDir); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	// Both directories should exist (no stripping)
	assertFileContent(t, filepath.Join(dstDir, "dir1", "file1.txt"), "content1")
	assertFileContent(t, filepath.Join(dstDir, "dir2", "file2.txt"), "content2")
}

func TestCreate_TarGz(t *testing.T) {
	// Create source directory
	srcDir := t.TempDir()
	testDir := filepath.Join(srcDir, "myapp")
	if err := os.MkdirAll(filepath.Join(testDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "bin", "app"), []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create archive
	path, err := Create(testDir, "linux", "amd64")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify path format
	expected := filepath.Join(srcDir, "myapp-linux-amd64.tar.gz")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("archive not created: %v", err)
	}
}

func TestCreate_Zip(t *testing.T) {
	// Create source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "app.exe")
	if err := os.WriteFile(srcFile, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create archive
	path, err := Create(srcFile, "windows", "amd64")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify path format
	expected := filepath.Join(srcDir, "app.exe-windows-amd64.zip")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

// Helper functions

func createTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read %q: %v", path, err)
		return
	}
	if string(data) != want {
		t.Errorf("file %q content = %q, want %q", path, string(data), want)
	}
}
