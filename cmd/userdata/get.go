package userdata

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetUserDataOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single team user data entry by its ID.\n",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Get a team user data entry",
		Example: `  lsh user-data get ud_xxxxxxxx`,
		Use:     "get <id>",
	}

	return cmd
}

type GetUserDataOperation struct{}

func (o *GetUserDataOperation) run(cmd *cobra.Command, args []string) error {
	userDataID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.UserData.Retrieve(ctx, userDataID, nil, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.UserDataObject == nil || response.UserDataObject.Data == nil {
		return fmt.Errorf("user data %q not found", userDataID)
	}

	if !lsh.Debug {
		entry := UserData{UserDataProperties: *response.UserDataObject.Data}
		utils.RenderDetails(entry.GetData())
	}

	return nil
}
