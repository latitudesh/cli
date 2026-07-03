package userdata

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListUserDataOperation{}
	cmd := &cobra.Command{
		Long: "List all user data in the team.\n\n" +
			"User data listed here belongs to the team (account scope). Use\n" +
			"`lsh projects user-data list` to see a project's user data.",
		RunE:    op.run,
		Short:   "List team user data",
		Example: `  lsh user-data list`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	return cmd
}

type ListUserDataOperation struct{}

func (o *ListUserDataOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	// The API paginates (defaulting to 20 items). This endpoint carries
	// x-speakeasy-pagination, so the SDK exposes a Next closure and we can
	// drive the shared pagination.Walk like the other migrated lists.
	opts := pagination.Resolve()
	request := operations.GetUsersDataRequest{PageSize: &opts.PageSize}

	first, err := client.UserData.List(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	list := UserDataList{}
	result, err := pagination.Walk(first, opts,
		func(r *operations.GetUsersDataResponse) func() (*operations.GetUsersDataResponse, error) {
			return r.Next
		},
		func(r *operations.GetUsersDataResponse, limit int) int {
			if r.UserData == nil {
				return 0
			}
			added := 0
			for i := range r.UserData.Data {
				if limit >= 0 && added >= limit {
					break
				}
				list.Data = append(list.Data, &UserData{UserDataProperties: r.UserData.Data[i]})
				added++
			}
			return added
		},
	)
	if err != nil {
		utils.PrintError(err)
		return err
	}
	if opts.NoPaginate && result.HasMore {
		pagination.PrintNextCursor(result.NextPage)
	}

	if !lsh.Debug {
		utils.Render(list.GetData())
	}

	return nil
}
