package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/client/servers"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	"github.com/spf13/cobra"
)

// waitForServerState blocks until the server reaches one of want, hits one of
// fail, times out, or the user cancels — but only when --wait was passed.
//
// The create/reinstall calls still go through the legacy client; the wait loop
// polls via the SDK (Servers.Get). Progress and outcome are written to stderr
// so they never corrupt structured (-o json) output on stdout.
func waitForServerState(cmd *cobra.Command, serverID string, want, fail []components.ServerDataStatus) error {
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

	fmt.Fprintf(os.Stderr, "Waiting for server %s to finish provisioning… (Ctrl+C to stop)\n", serverID)

	status, err := wait.ForServerState(ctx, client, serverID, want, fail, true, o, nil, operations.WithRetries(lsh.RetryConfig()))
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

	// The create/reinstall payload is an optimistic snapshot (it can report
	// "on" mid-deploy); render the real, final state by re-fetching the server.
	if !lsh.Debug {
		renderServerState(cmd, serverID)
	}
	return nil
}

// renderServerState re-fetches the server and renders its current state without
// the interactive table. Best-effort: a failed re-fetch is silent because the
// wait itself already succeeded.
func renderServerState(cmd *cobra.Command, serverID string) {
	appCli, err := makeClient(cmd, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not render final server state: %v\n", err)
		return
	}
	params := servers.NewGetServerParams()
	params.SetServerID(serverID)
	resp, err := appCli.Servers.GetServer(params, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not fetch final server state: %v\n", err)
		return
	}
	utils.RenderStatic(resp.GetData())
}

// serverProvisionTargets are the terminal states for a create/reinstall wait.
// Provisioning is "done" once the server settles into a stable power state —
// it may finish either powered on or off — and "failed" on a failed deployment.
// The in-progress states (deploying, disk_erasing) keep the wait polling.
func serverProvisionTargets() (want, fail []components.ServerDataStatus) {
	return []components.ServerDataStatus{
			components.ServerDataStatusOn,
			components.ServerDataStatusOff,
		},
		[]components.ServerDataStatus{components.ServerDataStatusFailedDeployment}
}
