// Package util holds small cross-cutting helpers shared by the
// command layer and the prompt/auth subpackages.
package util

import "os"

// IsTTY reports whether stdin is connected to a terminal.
// Used to decide whether to run interactive prompts vs. fail with
// an actionable error in non-interactive contexts (CI, pipes).
func IsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
