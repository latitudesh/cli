package projectuserdata

import (
	"context"
	"fmt"
	"net/http"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewDeleteCmd() *cobra.Command {
	op := DeleteProjectUserDataOperation{}
	cmd := &cobra.Command{
		Long:    "Delete a project user data entry by its ID.\n\nDefaults to the active project when --project is omitted.",
		RunE:    op.run,
		Args:    cobra.ExactArgs(1),
		Short:   "Delete a project user data entry",
		Example: `  lsh projects user-data delete ud_xxxxxxxx --project my-project`,
		Use:     "delete <id>",
		Aliases: []string{"rm"},
	}

	registerProjectFlag(cmd)

	return cmd
}

type DeleteProjectUserDataOperation struct{}

func (o *DeleteProjectUserDataOperation) run(cmd *cobra.Command, args []string) error {
	userDataID := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.UserData.DeleteProjectUserData(ctx, projectID(cmd), userDataID, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if !lsh.Debug {
		// The API answers deletes with 200 or 204 depending on the path.
		status := 0
		if resp.HTTPMeta.Response != nil {
			status = resp.HTTPMeta.Response.StatusCode
		}
		if status == http.StatusOK || status == http.StatusNoContent {
			fmt.Printf("\nUser data deleted successfully!\n")
		} else {
			fmt.Printf("Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
