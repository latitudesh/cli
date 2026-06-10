package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

var fetchSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// StartFetchSpinner shows a lightweight spinner on stderr while a command
// fetches data, so slow (multi-page) listings don't look frozen. It writes
// to stderr so piped stdout (json/table output) stays clean, and becomes a
// no-op when stderr is not a terminal (scripts, CI). The returned stop
// function clears the spinner line and is safe to call more than once.
func StartFetchSpinner(message string) (stop func()) {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			case <-ticker.C:
				frame := fetchSpinnerFrames[i%len(fetchSpinnerFrames)]
				fmt.Fprintf(os.Stderr, "\r%s %s", frame, message)
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(done)
			fmt.Fprintf(os.Stderr, "\r\033[K")
		})
	}
}
