package projectsshkeys

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/cmd/sshkeys"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetProjectSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single SSH key attached to a project.\n\nDefaults to the active project when --project is omitted.",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Get a project SSH key",
		Example: `  lsh projects ssh-keys get ssh_xxxxxxxx --project my-project`,
		Use:     "get <id>",
	}

	registerProjectFlag(cmd)

	return cmd
}

type GetProjectSSHKeyOperation struct{}

func (o *GetProjectSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	sshKeyID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.SSHKeys.Get(ctx, projectID(cmd), sshKeyID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object == nil || response.Object.Data == nil {
		return fmt.Errorf("SSH key %q not found in project", sshKeyID)
	}

	if !lsh.Debug {
		key := sshkeys.SSHKey{SSHKeyData: *response.Object.Data}
		utils.RenderDetails(key.GetData())
	}

	return nil
}
