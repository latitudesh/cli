package objectstorage

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/exitcode"
)

// withPrompt swaps the prompt streams for the duration of a test.
func withPrompt(t *testing.T, stdin string) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	inSaved, outSaved := promptIn, promptOut
	promptIn, promptOut = strings.NewReader(stdin), out
	t.Cleanup(func() { promptIn, promptOut = inSaved, outSaved })
	return out
}

// TestChooseManyLineCancelIsRefused covers the exit-code fix: aborting a
// selection is a refusal (7), not a usage error (2), and the numbered-list
// hint belongs to this fallback rather than to the callers' question text.
func TestChooseManyLineCancelIsRefused(t *testing.T) {
	out := withPrompt(t, "") // EOF: nothing typed
	idx, err := chooseManyLine("Which buckets should the key cover?", []string{"a", "b"})
	if idx != nil {
		t.Errorf("selected %v on EOF", idx)
	}
	if code := exitcode.Of(err); code != exitcode.Refused {
		t.Errorf("exit code = %d, want %d", code, exitcode.Refused)
	}
	if !strings.Contains(out.String(), "comma-separated numbers") {
		t.Errorf("the fallback must explain how to answer, got %q", out.String())
	}
}

func TestChooseManyLineSelection(t *testing.T) {
	withPrompt(t, "1,3\n")
	idx, err := chooseManyLine("Which buckets?", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("chooseManyLine: %v", err)
	}
	if len(idx) != 2 || idx[0] != 0 || idx[1] != 2 {
		t.Errorf("selected %v, want [0 2]", idx)
	}
}

// TestHumanizeAPIInterruption covers the exit-code fix on the control plane:
// Ctrl-C must yield 130 whichever phase of a command it lands in.
func TestHumanizeAPIInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := HumanizeAPI(ctx.Err(), "project \"x\"")
	if code := exitcode.Of(err); code != exitcode.Interrupted {
		t.Errorf("exit code = %d, want %d (err=%v)", code, exitcode.Interrupted, err)
	}
	// The S3 path already behaved this way; the two must agree.
	if code := exitcode.Of(Humanize(ctx.Err(), nil, nil)); code != exitcode.Interrupted {
		t.Errorf("Humanize disagrees: %d", code)
	}
}
