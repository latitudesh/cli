package utils

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTarGz builds a .tar.gz with one regular file per name.
func writeTarGz(t *testing.T, path string, names ...string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	for _, name := range names {
		body := []byte("content of " + name)
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUntarExtractsRelease(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "lsh_Linux_x86_64.tar.gz")
	writeTarGz(t, archive, "lsh_Linux_x86_64/lsh", "lsh_Linux_x86_64/README.md")
	dest := t.TempDir()

	if err := Untar(archive, dest); err != nil {
		t.Fatalf("Untar: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "lsh_Linux_x86_64", "lsh"))
	if err != nil {
		t.Fatalf("extracted binary missing: %v", err)
	}
	if string(b) != "content of lsh_Linux_x86_64/lsh" {
		t.Fatalf("extracted content = %q", b)
	}
}

func TestUntarRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "evil.tar.gz")
	dest := filepath.Join(root, "dest")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"../escaped", "lsh/../../escaped"} {
		t.Run(name, func(t *testing.T) {
			writeTarGz(t, archive, name)
			err := Untar(archive, dest)
			if err == nil || !strings.Contains(err.Error(), "invalid file path") {
				t.Fatalf("expected an invalid file path error, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "escaped")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("entry escaped the destination, stat err = %v", err)
			}
		})
	}
}
