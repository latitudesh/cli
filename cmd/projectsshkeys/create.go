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

func NewCreateCmd() *cobra.Command {
	op := CreateProjectSSHKeyOperation{}
	cmd := &cobra.Command{
		Long: "Create an SSH key scoped to a project.\n\n" +
			"The key is created inside the project from its name and public key\n" +
			"material — the API has no operation to attach an existing team key.\n" +
			"Defaults to the active project when --project is omitted.",
		RunE:  op.run,
		Short: "Create an SSH key in a project",
		Example: `  lsh projects ssh-keys create --name laptop --public-key "ssh-ed25519 AAAA..."
  lsh projects ssh-keys create --project my-project --name laptop --public-key "$(cat ~/.ssh/id_ed25519.pub)"`,
		Use: "create",
	}

	registerProjectFlag(cmd)
	cmd.Flags().String("name", "", "Name of the SSH key (required)")
	cmd.Flags().String("public-key", "", "SSH public key contents (required)")

	return cmd
}

type CreateProjectSSHKeyOperation struct{}

func (o *CreateProjectSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
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

	request := operations.PostProjectSSHKeyProjectsSSHKeysRequestBody{
		Data: operations.PostProjectSSHKeyProjectsSSHKeysData{
			Type: operations.PostProjectSSHKeyProjectsSSHKeysTypeSSHKeys,
			Attributes: &operations.PostProjectSSHKeyProjectsSSHKeysAttributes{
				Name:      &name,
				PublicKey: &publicKey,
			},
		},
	}

	response, err := client.Projects.SSHKeys.PostProjectSSHKey(ctx, projectID(cmd), request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Object != nil && response.Object.Data != nil && !lsh.Debug {
		key := sshkeys.SSHKey{SSHKeyData: *response.Object.Data}
		utils.RenderStatic(key.GetData())
	}

	return nil
}
