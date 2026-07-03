package sshkeys

import (
	"context"
	"fmt"
	"net/http"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	op := DeleteSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:    "Delete a team SSH key by its ID.\n",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a team SSH key",
		Example: `  lsh ssh-keys delete ssh_xxxxxxxx`,
		Use:     "delete <id>",
		Aliases: []string{"rm"},
	}

	return cmd
}

type DeleteSSHKeyOperation struct{}

func (o *DeleteSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	sshKeyID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.SSHKeys.Delete(ctx, sshKeyID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		// The API answers deletes with 200 or 204 depending on the path.
		status := 0
		if resp.HTTPMeta.Response != nil {
			status = resp.HTTPMeta.Response.StatusCode
		}
		if status == http.StatusOK || status == http.StatusNoContent {
			fmt.Printf("\nSSH key deleted successfully!\n")
		} else {
			fmt.Printf("Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
