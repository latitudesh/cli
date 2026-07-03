package servers

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

// NewLockCmd builds `lsh servers lock <id>`.
func NewLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "lock <server_id>",
		Short:   "Lock a server",
		Long:    "Lock a server to prevent destructive actions (deletion, reinstall, power changes).",
		Example: `  lsh servers lock sv_xxxxxxxx`,
		Args:    cobra.ExactArgs(1),
		RunE:    runLock,
	}
}

func runLock(cmd *cobra.Command, args []string) error {
	serverID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	if _, err := client.Servers.Lock(ctx, serverID, operations.WithRetries(lsh.RetryConfig())); err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		model := &SimpleServerModel{ServerID: serverID, State: "locked"}
		utils.RenderStatic(model.GetData())
	}
	return nil
}

// NewUnlockCmd builds `lsh servers unlock <id>`.
func NewUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "unlock <server_id>",
		Short:   "Unlock a server",
		Long:    "Unlock a server to allow destructive actions again.",
		Example: `  lsh servers unlock sv_xxxxxxxx`,
		Args:    cobra.ExactArgs(1),
		RunE:    runUnlock,
	}
}

func runUnlock(cmd *cobra.Command, args []string) error {
	serverID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	if _, err := client.Servers.Unlock(ctx, serverID, operations.WithRetries(lsh.RetryConfig())); err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		model := &SimpleServerModel{ServerID: serverID, State: "unlocked"}
		utils.RenderStatic(model.GetData())
	}
	return nil
}
