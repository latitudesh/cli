// Package wait provides reusable polling infrastructure shared by the
// `--wait` flag (block until a resource reaches a target state) and the
// `events list --follow` stream. It owns three concerns: a context-aware
// polling loop with bounded backoff, the `--wait`/`--timeout` flag pair, and
// a signal-derived context so Ctrl+C exits cleanly (no traceback).
package wait

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Sentinel errors returned by Poll and its callers. Commands map ErrTimeout to
// a non-zero exit code; ErrCanceled is the clean Ctrl+C path.
var (
	ErrTimeout  = errors.New("timed out waiting for the resource to reach its target state")
	ErrCanceled = errors.New("wait canceled")
)

// pollFloor is the minimum delay between two polls. It enforces the
// "≤ 1 req/s under steady state" budget no matter how the backoff is tuned.
const pollFloor = time.Second

// Backoff controls the delay between polls. The delay grows geometrically from
// Initial by Factor, capped at Max, and is never shorter than pollFloor.
type Backoff struct {
	Initial time.Duration
	Max     time.Duration
	Factor  float64
}

// DefaultBackoff is tuned for resource-state waiters: a quick first re-check
// that eases off so a long provision doesn't hammer the API.
func DefaultBackoff() Backoff {
	return Backoff{Initial: 2 * time.Second, Max: 15 * time.Second, Factor: 1.5}
}

// next returns the delay that should follow a poll whose preceding delay was
// prev. The first delay (prev <= 0) is Initial.
func (b Backoff) next(prev time.Duration) time.Duration {
	var d time.Duration
	if prev <= 0 {
		d = b.Initial
	} else {
		d = time.Duration(float64(prev) * b.Factor)
	}
	if d < pollFloor {
		d = pollFloor
	}
	if b.Max > 0 && d > b.Max {
		d = b.Max
	}
	return d
}

// Poll calls probe immediately, then repeatedly with a backoff delay between
// attempts, until probe reports done, probe returns an error, or ctx is
// cancelled/expired. A (false, nil) result means "not there yet — keep
// waiting". Context cancellation is normalized to ErrCanceled/ErrTimeout so
// callers get a stable, friendly error regardless of where it was observed.
func Poll(ctx context.Context, b Backoff, probe func(context.Context) (done bool, err error)) error {
	var delay time.Duration
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return classify(ctx.Err())
			case <-timer.C:
			}
		} else if err := ctx.Err(); err != nil {
			// Honor an already-cancelled context before the first probe.
			return classify(err)
		}

		done, err := probe(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		delay = b.next(delay)
	}
}

// classify maps raw context errors to this package's sentinels.
func classify(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ErrTimeout
	case errors.Is(err, context.Canceled):
		return ErrCanceled
	default:
		return err
	}
}

// Options is the parsed form of the --wait/--timeout flags.
type Options struct {
	Enabled bool
	Timeout time.Duration
}

// AddFlags registers the --wait/--timeout pair on a state-changing command.
func AddFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("wait", false, "block until the resource reaches its target state")
	cmd.Flags().Duration("timeout", 10*time.Minute, "maximum time to wait when --wait is set (e.g. 30s, 5m)")
}

// OptionsFrom reads the --wait/--timeout flags registered by AddFlags.
func OptionsFrom(cmd *cobra.Command) Options {
	enabled, _ := cmd.Flags().GetBool("wait")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	return Options{Enabled: enabled, Timeout: timeout}
}

// SignalContext derives a context that is cancelled on SIGINT (Ctrl+C) or
// SIGTERM, so long-running waits and the --follow stream stop cleanly. The
// returned cancel func must be called to release the signal handler.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
