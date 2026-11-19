package objectstorage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/latitudesh/lsh/internal/exitcode"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// stdin/stderr are package variables so tests can substitute them.
var (
	promptIn  io.Reader = os.Stdin
	promptOut io.Writer = os.Stderr
	isTTY               = func() bool {
		return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
	}
)

// CanPrompt reports whether the command may ask the user something: --no-input
// was not given, and both stdin and stderr are terminals. Prompts are written
// to stderr so stdout stays clean for pipes.
func CanPrompt(cmd *cobra.Command) bool {
	if cmd != nil {
		if noInput, err := cmd.Flags().GetBool("no-input"); err == nil && noInput {
			return false
		}
	}
	return isTTY()
}

// Confirm asks a yes/no question. In a real terminal it uses the shared
// Bubble Tea confirm widget (the same y/n prompt as the rest of the CLI);
// otherwise (tests, redirected streams) it falls back to a plain [y/N] line so
// the flow stays scriptable. EOF counts as No.
func Confirm(question string) (bool, error) {
	if isTTY() {
		return tui.RunConfirm(question)
	}
	return confirmLine(question)
}

// confirmLine is the non-interactive fallback of Confirm.
func confirmLine(question string) (bool, error) {
	fmt.Fprintf(promptOut, "? %s [y/N] ", question)
	line, err := readLine()
	if err != nil && line == "" {
		fmt.Fprintln(promptOut)
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// Choose lets the user pick one option and returns its index (0-based), or -1
// when the selection is cancelled. In a real terminal it uses the shared
// Bubble Tea list (arrow keys, filtering — the same picker as the rest of the
// CLI); otherwise it falls back to a numbered menu read from stdin.
func Choose(question string, options []string, defaultIndex int) (int, error) {
	if isTTY() {
		choice, err := tui.RunList(question, options, nil)
		if err != nil {
			// Cancelled (esc/ctrl+c) or no selection.
			return -1, nil
		}
		for i, opt := range options {
			if opt == choice {
				return i, nil
			}
		}
		return -1, nil
	}
	return chooseLine(question, options, defaultIndex)
}

// chooseLine is the non-interactive fallback of Choose.
func chooseLine(question string, options []string, defaultIndex int) (int, error) {
	fmt.Fprintln(promptOut, "? "+question)
	for i, opt := range options {
		marker := " "
		if i == defaultIndex {
			marker = "*"
		}
		fmt.Fprintf(promptOut, "  %s[%d] %s\n", marker, i+1, opt)
	}
	fmt.Fprintf(promptOut, "> ")
	line, err := readLine()
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		fmt.Fprintln(promptOut)
		return -1, nil
	}
	if line == "" {
		return defaultIndex, nil
	}
	n, convErr := strconv.Atoi(line)
	if convErr != nil || n < 1 || n > len(options) {
		return -1, nil
	}
	return n - 1, nil
}

// ReadSecret reads a secret from stdin without echo when stdin is a terminal,
// or a single line otherwise (pipes, heredocs). cmd is used to honour
// --no-input: a session that forbids prompts must fail instead of blocking on
// a terminal read. Piping the value on stdin keeps working either way.
func ReadSecret(cmd *cobra.Command, prompt string) (string, error) {
	if f, ok := promptIn.(*os.File); ok && term.IsTerminal(int(f.Fd())) && !CanPrompt(cmd) {
		return "", exitcode.Errorf(exitcode.Refused, "a secret is required but prompts are disabled; pipe it on stdin instead: echo \"$SECRET\" | lsh ...")
	}
	fmt.Fprint(promptOut, prompt)
	if f, ok := promptIn.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(promptOut)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := readLine()
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func readLine() (string, error) {
	r := bufio.NewReader(promptIn)
	line, err := r.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

// ConfirmOrRefuse implements the confirmation contract of destructive
// commands: --yes skips the prompt; otherwise an interactive session is asked
// and a non-interactive one fails fast with exit 7 instead of hanging.
func ConfirmOrRefuse(cmd *cobra.Command, yes bool, question string) error {
	if yes {
		return nil
	}
	if !CanPrompt(cmd) {
		return exitcode.Errorf(exitcode.Refused, "%s — refusing to continue without --yes in a non-interactive session", question)
	}
	ok, err := Confirm(question)
	if err != nil {
		return err
	}
	if !ok {
		return exitcode.Errorf(exitcode.Refused, "cancelled")
	}
	return nil
}

// Warnf prints a warning line to stderr.
func Warnf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", a...)
}

// Hintf prints an informational line to stderr (never stdout).
func Hintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// ReadLine prints a prompt on stderr and reads one line from stdin (the
// empty string on EOF). Use it for free-text answers such as names.
func ReadLine(prompt string) (string, error) {
	fmt.Fprint(promptOut, "? "+prompt)
	line, err := readLine()
	if err != nil && line == "" {
		fmt.Fprintln(promptOut)
		return "", nil
	}
	return strings.TrimSpace(line), nil
}

// ChooseMany lets the user pick several options and returns their indices. In
// a real terminal it uses the shared Bubble Tea checkbox list (space to toggle,
// enter to confirm); otherwise it falls back to reading a comma/space separated
// list of numbers. Aborting (esc/ctrl+c, or EOF in the fallback) is a refusal,
// not a usage error; a confirmed but empty selection returns no indices.
func ChooseMany(question string, options []string) ([]int, error) {
	if isTTY() {
		idx, err := tui.RunMultiSelect(question, options, nil)
		if errors.Is(err, tui.ErrCanceled) {
			return nil, exitcode.Errorf(exitcode.Refused, "cancelled")
		}
		return idx, err
	}
	return chooseManyLine(question, options)
}

// chooseManyLine is the non-interactive fallback of ChooseMany.
func chooseManyLine(question string, options []string) ([]int, error) {
	// The numbered list only exists in this fallback, so the hint about typing
	// numbers belongs here and not in the callers' question text.
	fmt.Fprintln(promptOut, "? "+question+" (comma-separated numbers)")
	for i, opt := range options {
		fmt.Fprintf(promptOut, "   [%d] %s\n", i+1, opt)
	}
	fmt.Fprint(promptOut, "> ")
	line, err := readLine()
	if err != nil && strings.TrimSpace(line) == "" {
		fmt.Fprintln(promptOut)
		return nil, exitcode.Errorf(exitcode.Refused, "cancelled")
	}
	seen := map[int]bool{}
	var out []int
	for _, tok := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		n, convErr := strconv.Atoi(strings.TrimSpace(tok))
		if convErr != nil || n < 1 || n > len(options) {
			return nil, nil
		}
		if !seen[n-1] {
			seen[n-1] = true
			out = append(out, n-1)
		}
	}
	return out, nil
}
