package projectsshkeys

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
	op := DeleteProjectSSHKeyOperation{}
	cmd := &cobra.Command{
		Long: "Delete an SSH key from a project.\n\n" +
			"The key is deleted entirely (not just dissociated from the project) and\n" +
			"is also cleaned from the servers' authorized keys.\n" +
			"Defaults to the active project when --project is omitted.",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a project SSH key",
		Example: `  lsh projects ssh-keys delete ssh_xxxxxxxx --project my-project`,
		Use:     "delete <id>",
		Aliases: []string{"rm"},
	}

	registerProjectFlag(cmd)

	return cmd
}

type DeleteProjectSSHKeyOperation struct{}

func (o *DeleteProjectSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	sshKeyID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.SSHKeys.RemoveFromProject(ctx, projectID(cmd), sshKeyID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		// The API answers this delete with 200 or 204 depending on the path.
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
