package userdata

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	op := UpdateUserDataOperation{}
	cmd := &cobra.Command{
		Long:  "Update a team user data entry. Only the flags you pass are changed.\n",
		RunE:  op.run,
		Args:  cobra.ExactArgs(1),
		Short: "Update a team user data entry",
		Example: `  lsh user-data update ud_xxxxxxxx --description renamed
  lsh user-data update ud_xxxxxxxx --content "#cloud-config\n..."`,
		Use: "update <id>",
	}

	cmd.Flags().String("description", "", "New description of the user data")
	RegisterContentFlags(cmd)

	return cmd
}

type UpdateUserDataOperation struct{}

func (o *UpdateUserDataOperation) run(cmd *cobra.Command, args []string) error {
	userDataID := args[0]

	attributes := &operations.PatchUserDataUserDataAttributes{}
	changed := false
	if cmd.Flags().Changed("description") {
		description, _ := cmd.Flags().GetString("description")
		attributes.Description = &description
		changed = true
	}
	if content, ok := ResolveContent(cmd); ok {
		attributes.Content = &content
		changed = true
	}
	if !changed {
		return fmt.Errorf("provide at least one of --description or --content/--content-base64 to update")
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.PatchUserDataUserDataRequestBody{
		Data: operations.PatchUserDataUserDataData{
			ID:         userDataID,
			Type:       operations.PatchUserDataUserDataTypeUserData,
			Attributes: attributes,
		},
	}

	response, err := client.UserData.Update(ctx, userDataID, &request, operations.WithRetries(lsh.RetryConfig()))
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
