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

func NewUpdateCmd() *cobra.Command {
	op := UpdateProjectUserDataOperation{}
	cmd := &cobra.Command{
		Long:  "Update a project user data entry. Only the flags you pass are changed.\n\nDefaults to the active project when --project is omitted.",
		RunE:  op.run,
		Args:  cobra.ExactArgs(1),
		Short: "Update a project user data entry",
		Example: `  lsh projects user-data update ud_xxxxxxxx --description renamed
  lsh projects user-data update ud_xxxxxxxx --project my-project --content "#cloud-config\n..."`,
		Use: "update <id>",
	}

	registerProjectFlag(cmd)
	cmd.Flags().String("description", "", "New description of the user data")
	userdata.RegisterContentFlags(cmd)

	return cmd
}

type UpdateProjectUserDataOperation struct{}

func (o *UpdateProjectUserDataOperation) run(cmd *cobra.Command, args []string) error {
	userDataID := args[0]

	attributes := &operations.PutProjectUserDataUserDataAttributes{}
	changed := false
	if cmd.Flags().Changed("description") {
		description, _ := cmd.Flags().GetString("description")
		attributes.Description = &description
		changed = true
	}
	if content, ok := userdata.ResolveContent(cmd); ok {
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

	request := operations.PutProjectUserDataUserDataRequestBody{
		Data: operations.PutProjectUserDataUserDataData{
			ID:         userDataID,
			Type:       operations.PutProjectUserDataUserDataTypeUserData,
			Attributes: attributes,
		},
	}

	response, err := client.UserData.UpdateForProject(ctx, projectID(cmd), userDataID, &request, operations.WithRetries(lsh.RetryConfig()))
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
