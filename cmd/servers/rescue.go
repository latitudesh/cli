package servers

import (
	"context"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	cobra "github.com/spf13/cobra"
)

// NewRescueModeCmd builds `lsh servers rescue-mode <id>`.
func NewRescueModeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rescue-mode <server_id>",
		Short: "Boot a server into rescue mode",
		Long: "Reboot a server into rescue mode.\n\n" +
			"With --wait the command blocks until the server settles into the\n" +
			"rescue_mode state.\n\n" +
			"The rescue login credentials are not exposed by the API; retrieve them\n" +
			"on the server's page in the dashboard.",
		Example: `  lsh servers rescue-mode sv_xxxxxxxx
  lsh servers rescue-mode sv_xxxxxxxx --wait`,
		Args: cobra.ExactArgs(1),
		RunE: runRescueMode,
	}
	wait.AddFlags(cmd)
	return cmd
}

func runRescueMode(cmd *cobra.Command, args []string) error {
	serverID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	if _, err := client.Servers.StartRescueMode(ctx, serverID, operations.WithRetries(lsh.RetryConfig())); err != nil {
		utils.PrintError(err)
		return err
	}

	waitEnabled := wait.OptionsFrom(cmd).Enabled
	if !lsh.Debug && !waitEnabled {
		model := &SimpleServerModel{ServerID: serverID, State: "rescue mode requested"}
		utils.RenderStatic(model.GetData())
	}
	// The API does not expose the rescue credentials; point the user at the
	// dashboard. stderr keeps stdout clean for structured output.
	fmt.Fprintln(os.Stderr, "Note: rescue login credentials are available on the server's page in the dashboard.")

	want := []components.ServerDataStatus{components.ServerDataStatusRescueMode}
	fail := []components.ServerDataStatus{components.ServerDataStatusFailedDeployment}
	// Idempotent wait: a server already in the target state is already done —
	// requiring a transition here would hang until timeout.
	return waitForServerState(cmd, serverID, want, fail, false)
}

// NewExitRescueModeCmd builds `lsh servers exit-rescue-mode <id>`.
func NewExitRescueModeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exit-rescue-mode <server_id>",
		Short: "Exit rescue mode on a server",
		Long: "Reboot a server out of rescue mode into its normal operating system.\n\n" +
			"With --wait the command blocks until the server settles into a stable\n" +
			"power state (on/off).",
		Example: `  lsh servers exit-rescue-mode sv_xxxxxxxx
  lsh servers exit-rescue-mode sv_xxxxxxxx --wait`,
		Args: cobra.ExactArgs(1),
		RunE: runExitRescueMode,
	}
	wait.AddFlags(cmd)
	return cmd
}

func runExitRescueMode(cmd *cobra.Command, args []string) error {
	serverID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	if _, err := client.Servers.ExitRescueMode(ctx, serverID, operations.WithRetries(lsh.RetryConfig())); err != nil {
		utils.PrintError(err)
		return err
	}

	waitEnabled := wait.OptionsFrom(cmd).Enabled
	if !lsh.Debug && !waitEnabled {
		model := &SimpleServerModel{ServerID: serverID, State: "exit rescue mode requested"}
		utils.RenderStatic(model.GetData())
	}

	// Leaving rescue mode reboots into the installed OS; it settles on/off.
	want := []components.ServerDataStatus{
		components.ServerDataStatusOn,
		components.ServerDataStatusOff,
	}
	fail := []components.ServerDataStatus{components.ServerDataStatusFailedDeployment}
	// Idempotent wait: a server already in the target state is already done —
	// requiring a transition here would hang until timeout.
	return waitForServerState(cmd, serverID, want, fail, false)
}
