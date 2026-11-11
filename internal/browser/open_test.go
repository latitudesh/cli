package browser

import (
	"os"
	"runtime"
	"testing"
)

func clearHeadlessEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SSH_TTY", "")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")
}

func TestLooksHeadlessOverSSH(t *testing.T) {
	clearHeadlessEnv(t)
	t.Setenv("SSH_CONNECTION", "10.0.0.1 50000 10.0.0.2 22")

	if !LooksHeadless() {
		t.Fatal("expected an SSH session to look headless")
	}
}

func TestLooksHeadlessWithPipedStdin(t *testing.T) {
	clearHeadlessEnv(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev })

	if LooksHeadless() {
		t.Fatal("piped stdin with a display must not look headless")
	}
}

func TestLooksHeadlessLinuxWithoutDisplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("display detection only applies on linux")
	}
	clearHeadlessEnv(t)
	t.Setenv("DISPLAY", "")

	if !LooksHeadless() {
		t.Fatal("expected linux without a display to look headless")
	}
}
