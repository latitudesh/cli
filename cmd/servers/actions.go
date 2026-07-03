package servers

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	cobra "github.com/spf13/cobra"
)

// validActions maps a CLI power-action name to the SDK enum accepted by
// POST /servers/{id}/actions.
var validActions = map[string]operations.CreateServerActionAction{
	"power_on":  operations.CreateServerActionActionPowerOn,
	"power_off": operations.CreateServerActionActionPowerOff,
	"reboot":    operations.CreateServerActionActionReboot,
}

// newPowerActionCmd builds a `lsh servers <use> <server_id>` command that runs a
// single power action, following the gcloud/doctl convention of a dedicated
// subcommand per action instead of a generic `actions --action <name>`.
func newPowerActionCmd(use, short, long, example, action string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use + " <server_id>",
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPowerAction(cmd, args[0], action)
		},
	}
	wait.AddFlags(cmd)
	return cmd
}

// NewPowerOnCmd builds `lsh servers power-on <server_id>`.
func NewPowerOnCmd() *cobra.Command {
	return newPowerActionCmd(
		"power-on",
		"Power on a server",
		"Power on a server.\n\nWith --wait the command blocks until the server is on.",
		`  lsh servers power-on sv_xxxxxxxx
  lsh servers power-on sv_xxxxxxxx --wait`,
		"power_on",
	)
}

// NewPowerOffCmd builds `lsh servers power-off <server_id>`.
func NewPowerOffCmd() *cobra.Command {
	return newPowerActionCmd(
		"power-off",
		"Power off a server",
		"Power off a server.\n\nWith --wait the command blocks until the server is off.",
		`  lsh servers power-off sv_xxxxxxxx
  lsh servers power-off sv_xxxxxxxx --wait`,
		"power_off",
	)
}

// NewRebootCmd builds `lsh servers reboot <server_id>`.
func NewRebootCmd() *cobra.Command {
	return newPowerActionCmd(
		"reboot",
		"Reboot a server",
		"Reboot a server.\n\nWith --wait the command blocks until the server is back on.",
		`  lsh servers reboot sv_xxxxxxxx
  lsh servers reboot sv_xxxxxxxx --wait`,
		"reboot",
	)
}

// runPowerAction sends the power action to the API and, unless --wait is set,
// renders the result statically. With --wait it blocks until the server reaches
// the expected power state.
func runPowerAction(cmd *cobra.Command, serverID, action string) error {
	sdkAction, ok := validActions[action]
	if !ok {
		err := fmt.Errorf("unknown power action %q", action)
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	requestBody := operations.CreateServerActionServersRequestBody{
		Data: operations.CreateServerActionServersData{
			Type: operations.CreateServerActionServersTypeActions,
			Attributes: &operations.CreateServerActionServersAttributes{
				Action: sdkAction,
			},
		},
	}

	response, err := client.Servers.RunAction(ctx, serverID, requestBody, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	waitEnabled := wait.OptionsFrom(cmd).Enabled
	if !lsh.Debug && !waitEnabled {
		data := &ServerActionModel{ServerID: serverID, Action: action}
		if response.ServerAction != nil {
			data.Data = response.ServerAction.Data
		}
		utils.RenderStatic(data.GetData())
	}

	want, fail := powerActionTargets(action)
	// Only reboot must observe a transition (the server is "on" before and
	// after); power_on/power_off on a server already in the target state are
	// already done, and requiring a transition would hang until timeout.
	return waitForServerState(cmd, serverID, want, fail, action == "reboot")
}
