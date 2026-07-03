package servers

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/wait"
	"github.com/spf13/cobra"
)

// waitForServerState blocks until the server reaches one of want, hits one of
// fail, times out, or the user cancels — but only when --wait was passed. All
// progress is written to stderr so it never corrupts structured (-o json)
// output on stdout.
//
// requireTransition guards operations that act on a server which may already
// sit in a target state (e.g. power_on on an already-on server) so the wait
// does not return before the operation has actually taken effect.
func waitForServerState(cmd *cobra.Command, serverID string, want, fail []components.ServerDataStatus, requireTransition bool) error {
	o := wait.OptionsFrom(cmd)
	if !o.Enabled {
		if cmd.Flags().Changed("timeout") {
			fmt.Fprintln(os.Stderr, "warning: --timeout has no effect without --wait")
		}
		return nil
	}
	if serverID == "" {
		fmt.Fprintln(os.Stderr, "warning: --wait ignored: could not determine the server ID")
		return nil
	}

	client := lsh.NewClient()
	ctx, cancel := wait.SignalContext(context.Background())
	defer cancel()

	fmt.Fprintf(os.Stderr, "Waiting for server %s to reach its target state… (Ctrl+C to stop)\n", serverID)

	status, err := wait.ForServerState(ctx, client, serverID, want, fail, requireTransition, o, nil, operations.WithRetries(lsh.RetryConfig()))
	switch {
	case errors.Is(err, wait.ErrCanceled):
		return fmt.Errorf("wait canceled (last status: %s)", status)
	case errors.Is(err, wait.ErrTimeout):
		return fmt.Errorf("timed out waiting for server %s (last status: %s)", serverID, status)
	case errors.Is(err, wait.ErrFailedState):
		return fmt.Errorf("server %s entered a failed state: %s", serverID, status)
	case err != nil:
		return err
	}

	fmt.Fprintf(os.Stderr, "Server %s is now %q\n", serverID, status)
	return nil
}

// powerActionTargets maps a power action to the server states that satisfy the
// wait (want) and the states that abort it (fail).
func powerActionTargets(action string) (want, fail []components.ServerDataStatus) {
	fail = []components.ServerDataStatus{components.ServerDataStatusFailedDeployment}
	switch action {
	case "power_on", "reboot":
		return []components.ServerDataStatus{components.ServerDataStatusOn}, fail
	case "power_off":
		return []components.ServerDataStatus{components.ServerDataStatusOff}, fail
	default:
		return nil, fail
	}
}
