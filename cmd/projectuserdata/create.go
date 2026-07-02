package projectuserdata

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/cmd/userdata"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateProjectUserDataOperation{}
	cmd := &cobra.Command{
		Long:  "Create a user data entry scoped to a project.\n\nDefaults to the active project when --project is omitted.",
		RunE:  op.run,
		Short: "Create a project user data entry",
		Example: `  lsh projects user-data create --description cloud-init --content "#cloud-config\n..."
  lsh projects user-data create --project my-project --description cloud-init --content-base64 "$(base64 < cloud-init.yaml)"`,
		Use: "create",
	}

	registerProjectFlag(cmd)
	cmd.Flags().String("description", "", "Description of the user data (required)")
	registerContentFlags(cmd)

	return cmd
}

type CreateProjectUserDataOperation struct{}

func (o *CreateProjectUserDataOperation) run(cmd *cobra.Command, args []string) error {
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

	request := operations.PostProjectUserDataUserDataRequestBody{
		Data: operations.PostProjectUserDataUserDataData{
			Type: operations.PostProjectUserDataUserDataTypeUserData,
			Attributes: &operations.PostProjectUserDataUserDataAttributes{
				Description: description,
				Content:     content,
			},
		},
	}

	response, err := client.UserData.Create(ctx, projectID(cmd), request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.UserDataObject != nil && response.UserDataObject.Data != nil && !lsh.Debug {
		entry := userdata.UserData{UserDataProperties: *response.UserDataObject.Data}
		utils.RenderStatic(entry.GetData())
	}

	return nil
}
