package sshkeys

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListSSHKeyOperation{}
	cmd := &cobra.Command{
		Long: "List all SSH keys in the team.\n\n" +
			"SSH keys listed here belong to the team (account scope). Use\n" +
			"`lsh projects ssh-keys list` to see the keys attached to a project.",
		RunE:  op.run,
		Short: "List team SSH keys",
		Example: `  lsh ssh-keys list
  lsh ssh-keys list --tags tag_xxxxxxxx
  lsh ssh-keys list -o json`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	cmd.Flags().String("tags", "", "Filter by tag ID")

	return cmd
}

type ListSSHKeyOperation struct{}

func (o *ListSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.GetSSHKeysRequest{}
	if tags, _ := cmd.Flags().GetString("tags"); tags != "" {
		request.FilterTags = &tags
	}

	// The API paginates (defaulting to 20 items), so walk every page — the SDK
	// exposes no Next() for this endpoint. Honors --page-size/--max-items/
	// --no-paginate like the other list commands.
	opts := pagination.Resolve()
	keys := SSHKeys{}
	for page := int64(1); ; page++ {
		request.PageSize = &opts.PageSize
		request.PageNumber = &page

		response, err := client.SSHKeys.ListAll(ctx, request, operations.WithRetries(lsh.RetryConfig()))
		if err != nil {
			utils.PrintError(err)
			return err
		}

		fetched := 0
		if response.SSHKeys != nil {
			fetched = len(response.SSHKeys.Data)
			for i := range response.SSHKeys.Data {
				if opts.MaxItems > 0 && int64(len(keys.Data)) >= opts.MaxItems {
					break
				}
				keys.Data = append(keys.Data, &SSHKey{SSHKeyData: response.SSHKeys.Data[i]})
			}
		}

		if opts.NoPaginate {
			if int64(fetched) == opts.PageSize {
				pagination.PrintNextCursor(page + 1)
			}
			break
		}
		if opts.MaxItems > 0 && int64(len(keys.Data)) >= opts.MaxItems {
			break
		}
		if int64(fetched) < opts.PageSize {
			break
		}
	}

	if !lsh.Debug {
		utils.Render(keys.GetData())
	}

	return nil
}
