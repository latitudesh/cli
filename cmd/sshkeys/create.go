package sshkeys

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:  "Create an SSH key in the team.\n",
		RunE:  op.run,
		Short: "Create a team SSH key",
		Example: `  lsh ssh-keys create --name laptop --public-key "ssh-ed25519 AAAA..."
  lsh ssh-keys create --name laptop --public-key "$(cat ~/.ssh/id_ed25519.pub)"`,
		Use: "create",
	}

	cmd.Flags().String("name", "", "Name of the SSH key (required)")
	cmd.Flags().String("public-key", "", "SSH public key contents (required)")

	return cmd
}

type CreateSSHKeyOperation struct{}

func (o *CreateSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	publicKey, _ := cmd.Flags().GetString("public-key")

	if name == "" || publicKey == "" {
		return fmt.Errorf("--name and --public-key are required")
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.PostSSHKeySSHKeysRequestBody{
		Data: operations.PostSSHKeySSHKeysData{
			Type: operations.PostSSHKeySSHKeysTypeSSHKeys,
			Attributes: &operations.PostSSHKeySSHKeysAttributes{
				Name:      &name,
				PublicKey: &publicKey,
			},
		},
	}

	response, err := client.SSHKeys.Create(ctx, request, operations.WithRetries(lsh.RetryConfig()))
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
