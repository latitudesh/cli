package sshkeys

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single team SSH key by its ID.\n",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Get a team SSH key",
		Example: `  lsh ssh-keys get ssh_xxxxxxxx`,
		Use:     "get <id>",
	}

	return cmd
}

type GetSSHKeyOperation struct{}

func (o *GetSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	sshKeyID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.SSHKeys.Retrieve(ctx, sshKeyID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object == nil || response.Object.Data == nil {
		return fmt.Errorf("SSH key %q not found", sshKeyID)
	}

	if !lsh.Debug {
		key := SSHKey{SSHKeyData: *response.Object.Data}
		utils.RenderDetails(key.GetData())
	}

	return nil
}
