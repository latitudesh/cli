package sshkeys

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	op := UpdateSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:  "Update a team SSH key. Only the flags you pass are changed.\n",
		RunE:  op.run,
		Args:  cobra.ExactArgs(1),
		Short: "Update a team SSH key",
		Example: `  lsh ssh-keys update ssh_xxxxxxxx --name renamed
  lsh ssh-keys update ssh_xxxxxxxx --tags tag_a --tags tag_b`,
		Use: "update <id>",
	}

	cmd.Flags().String("name", "", "New name of the SSH key")
	cmd.Flags().StringSlice("tags", nil, "Tag IDs to associate with the SSH key (repeatable)")

	return cmd
}

type UpdateSSHKeyOperation struct{}

func (o *UpdateSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	sshKeyID := args[0]

	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("tags") {
		return fmt.Errorf("provide at least one of --name or --tags to update")
	}

	// Only set attributes the user actually provided so a partial update patches
	// those fields instead of blanking the rest.
	attributes := &operations.PutSSHKeySSHKeysAttributes{}
	if cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		attributes.Name = &name
	}
	if cmd.Flags().Changed("tags") {
		tags, _ := cmd.Flags().GetStringSlice("tags")
		attributes.Tags = tags
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.PutSSHKeySSHKeysRequestBody{
		Data: operations.PutSSHKeySSHKeysData{
			ID:         &sshKeyID,
			Type:       operations.PutSSHKeySSHKeysTypeSSHKeys,
			Attributes: attributes,
		},
	}

	response, err := client.SSHKeys.Update(ctx, sshKeyID, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object != nil && response.Object.Data != nil && !lsh.Debug {
		key := SSHKey{SSHKeyData: *response.Object.Data}
		utils.RenderStatic(key.GetData())
	}

	return nil
}
