package userdata

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateUserDataOperation{}
	cmd := &cobra.Command{
		Long:  "Create a user data entry in the team.\n",
		RunE:  op.run,
		Short: "Create a team user data entry",
		Example: `  lsh user-data create --description cloud-init --content "#cloud-config\n..."
  lsh user-data create --description cloud-init --content-base64 "$(base64 < cloud-init.yaml)"`,
		Use: "create",
	}

	cmd.Flags().String("description", "", "Description of the user data (required)")
	registerContentFlags(cmd)

	return cmd
}

type CreateUserDataOperation struct{}

func (o *CreateUserDataOperation) run(cmd *cobra.Command, args []string) error {
	description, _ := cmd.Flags().GetString("description")
	content, ok := resolveContent(cmd)

	if description == "" || !ok {
		return fmt.Errorf("--description and one of --content/--content-base64 are required")
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.PostUserDataUserDataRequestBody{
		Data: operations.PostUserDataUserDataData{
			Type: operations.PostUserDataUserDataTypeUserData,
			Attributes: &operations.PostUserDataUserDataAttributes{
				Description: description,
				Content:     content,
			},
		},
	}

	response, err := client.UserData.CreateNew(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.UserDataObject != nil && response.UserDataObject.Data != nil && !lsh.Debug {
		entry := UserData{UserDataProperties: *response.UserDataObject.Data}
		utils.RenderStatic(entry.GetData())
	}

	return nil
}
