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
// incapable of opening a desktop browser (SSH session, no display,
// or non-TTY stdin). Callers use this to decide whether to attempt
// browser.Open at all — if headless, the URL is printed and the user
// opens it manually on their own machine.
func LooksHeadless() bool {
	if os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return true
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}
	if info, err := os.Stdin.Stat(); err == nil {
		if (info.Mode() & os.ModeCharDevice) == 0 {
			return true
		}
	}
	return false
}
