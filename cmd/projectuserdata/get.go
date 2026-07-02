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

func NewGetCmd() *cobra.Command {
	op := GetProjectUserDataOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single project user data entry by its ID.\n\nDefaults to the active project when --project is omitted.",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Get a project user data entry",
		Example: `  lsh projects user-data get ud_xxxxxxxx --project my-project`,
		Use:     "get <id>",
	}

	registerProjectFlag(cmd)

	return cmd
}

type GetProjectUserDataOperation struct{}

func (o *GetProjectUserDataOperation) run(cmd *cobra.Command, args []string) error {
	userDataID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.UserData.GetProjectUserData(ctx, projectID(cmd), userDataID, nil, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.UserDataObject == nil || response.UserDataObject.Data == nil {
		return fmt.Errorf("user data %q not found in project", userDataID)
	}

	if !lsh.Debug {
		entry := userdata.UserData{UserDataProperties: *response.UserDataObject.Data}
		utils.RenderDetails(entry.GetData())
	}

	return nil
}
