// Package util holds small cross-cutting helpers shared by the
// command layer and the prompt/auth subpackages.
package util

import (
	"os"

	"golang.org/x/term"
)

// IsTTY reports whether stdin is connected to a terminal.
// Used to decide whether to run interactive prompts vs. fail with
// an actionable error in non-interactive contexts (CI, pipes).
//
// term.IsTerminal (isatty) is used instead of checking ModeCharDevice:
// /dev/null is a character device but not a terminal, and prompting
// against it crashes bubbletea with "could not open a new TTY".
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
