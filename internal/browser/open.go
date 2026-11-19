// Package browser opens a URL in the user's default browser, with a
// hook so tests can substitute the real implementation.
package browser

import (
	"os"
	"runtime"

	"github.com/pkg/browser"
)

// Opener is the function used to open URLs. Tests can replace this
// with a no-op or capture function.
var Opener = browser.OpenURL

// Open invokes Opener for the given URL.
func Open(url string) error {
	return Opener(url)
}

// LooksHeadless reports whether the current environment seems
// incapable of opening a desktop browser (SSH session or no display).
func LooksHeadless() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return true
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}
	return false
}
