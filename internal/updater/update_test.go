package updater

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be gone, stat err = %v", path, err)
	}
}

// failRename makes rename fail for any move whose source is one of from.
func failRename(t *testing.T, from ...string) {
	t.Helper()
	orig := rename
	t.Cleanup(func() { rename = orig })
	rename = func(oldpath, newpath string) error {
		for _, f := range from {
			if oldpath == f {
				return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: errors.New("simulated failure")}
			}
		}
		return orig(oldpath, newpath)
	}
}

// The downloaded archive lives in the system temp dir, which may be another
// filesystem (tmpfs /tmp). Staging must land next to the target so the final
// swap is a same-filesystem rename.
func TestStageCopiesNextToTarget(t *testing.T) {
	src := filepath.Join(t.TempDir(), "lsh")
	writeFile(t, src, "new", 0o644)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "lsh")
	writeFile(t, target, "old", 0o750)

	staged, err := stage(src, target)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if filepath.Dir(staged) != targetDir {
		t.Fatalf("staged in %s, want %s", filepath.Dir(staged), targetDir)
	}
	if got := readFile(t, staged); got != "new" {
		t.Fatalf("staged content = %q, want %q", got, "new")
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target changed while staging: %q", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(staged)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o750 {
			t.Fatalf("staged mode = %v, want the target's 0750", info.Mode().Perm())
		}
	}
}

func TestStageFailsWhenTargetDirIsReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	src := filepath.Join(t.TempDir(), "lsh")
	writeFile(t, src, "new", 0o755)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "lsh")
	writeFile(t, target, "old", 0o755)
	if err := os.Chmod(targetDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(targetDir, 0o755) })

	if _, err := stage(src, target); err == nil || !strings.Contains(err.Error(), "prepare update") {
		t.Fatalf("expected a prepare update error, got %v", err)
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("target changed: %q", got)
	}
}

func TestVerifyVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the fake binary")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	writeFile(t, good, "#!/bin/sh\necho \"lsh v9.9.9\"\n", 0o755)
	broken := filepath.Join(dir, "broken")
	writeFile(t, broken, "#!/bin/sh\nexit 1\n", 0o755)

	cases := []struct {
		name    string
		path    string
		version string
		wantErr string
	}{
		{name: "matching version", path: good, version: "v9.9.9"},
		{name: "different version", path: good, version: "v9.9.1", wantErr: "expected v9.9.1"},
		{name: "prefix is not a match", path: good, version: "v9.9", wantErr: "expected v9.9"},
		{name: "binary fails to run", path: broken, version: "v9.9.9", wantErr: "failed to run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyVersion(tc.path, tc.version)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestReplaceInstallsStagedBinary(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "lsh")
	staged := filepath.Join(dir, ".lsh-update-1")
	writeFile(t, target, "old", 0o755)
	writeFile(t, staged, "new", 0o755)

	if err := replace(target, staged); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := readFile(t, target); got != "new" {
		t.Fatalf("target content = %q, want %q", got, "new")
	}
	assertMissing(t, staged)
	if runtime.GOOS != "windows" {
		assertMissing(t, target+"-old")
	}
}

func TestSwapWithBackupRestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "lsh")
	staged := filepath.Join(dir, ".lsh-update-1")
	backup := target + "-old"
	writeFile(t, target, "old", 0o755)
	writeFile(t, staged, "new", 0o755)
	failRename(t, staged)

	if err := swapWithBackup(target, staged, backup); err == nil {
		t.Fatal("expected the swap to fail")
	}
	if got := readFile(t, target); got != "old" {
		t.Fatalf("previous binary not restored, target = %q", got)
	}
	assertMissing(t, backup)
}

func TestSwapWithBackupReportsFailedRestore(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "lsh")
	staged := filepath.Join(dir, ".lsh-update-1")
	backup := target + "-old"
	writeFile(t, target, "old", 0o755)
	writeFile(t, staged, "new", 0o755)
	failRename(t, staged, backup)

	err := swapWithBackup(target, staged, backup)
	if err == nil || !strings.Contains(err.Error(), backup) {
		t.Fatalf("expected the error to point at the backup %s, got %v", backup, err)
	}
}

// install.sh links /usr/local/bin/lsh to ~/.lsh/lsh, and os.Executable keeps
// the link on macOS. The update must replace the real binary, not the link.
func TestResolveExecutableFollowsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need extra privileges on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "lsh")
	writeFile(t, real, "bin", 0o755)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got, err := resolveExecutable(link)
	if err != nil {
		t.Fatalf("resolveExecutable: %v", err)
	}
	want, _ := filepath.EvalSymlinks(real)
	if got != want {
		t.Fatalf("resolved to %s, want %s", got, want)
	}
}

func TestDownloadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("archive"))
	}))
	defer srv.Close()
	dir := t.TempDir()

	ok := filepath.Join(dir, "ok.tar.gz")
	if err := downloadFile(srv.URL+"/ok", ok); err != nil {
		t.Fatalf("download: %v", err)
	}
	if got := readFile(t, ok); got != "archive" {
		t.Fatalf("downloaded %q, want %q", got, "archive")
	}

	missing := filepath.Join(dir, "missing.tar.gz")
	err := downloadFile(srv.URL+"/missing", missing)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected a 404 error, got %v", err)
	}
	assertMissing(t, missing)
}
