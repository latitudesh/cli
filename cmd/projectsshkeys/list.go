package projectsshkeys

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/cmd/sshkeys"
	"github.com/latitudesh/lsh/internal/pagination"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListProjectSSHKeyOperation{}
	cmd := &cobra.Command{
		Long:  "List the SSH keys attached to a project.\n\nDefaults to the active project when --project is omitted.",
		RunE:  op.run,
		Short: "List project SSH keys",
		Example: `  lsh projects ssh-keys list
  lsh projects ssh-keys list --project my-project`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	registerProjectFlag(cmd)
	cmd.Flags().String("tags", "", "Filter by tag ID")

	return cmd
}

type ListProjectSSHKeyOperation struct{}

func (o *ListProjectSSHKeyOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	request := operations.GetProjectSSHKeysRequest{
		ProjectID: projectID(cmd),
	}
	if tags, _ := cmd.Flags().GetString("tags"); tags != "" {
		request.FilterTags = &tags
	}

	// The API paginates (defaulting to 20 items), so walk every page — the SDK
	// exposes no Next() for this endpoint. Honors --page-size/--max-items/
	// --no-paginate like the other list commands.
	opts := pagination.Resolve()
	keys := sshkeys.SSHKeys{}
	for page := int64(1); ; page++ {
		request.PageSize = &opts.PageSize
		request.PageNumber = &page

		response, err := client.SSHKeys.List(ctx, request, operations.WithRetries(lsh.RetryConfig()))
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
				keys.Data = append(keys.Data, &sshkeys.SSHKey{SSHKeyData: response.SSHKeys.Data[i]})
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
		// A friendly empty state for the human view: an empty project is not an
		// error, and the keys the user is looking for often live at team level.
		// Structured formats (-o json/yaml/csv) still render the empty list.
		if len(keys.Data) == 0 && renderer.ResolveFormat() == renderer.FormatTable {
			fmt.Printf("No SSH keys attached to project %q.\n", request.ProjectID)
			fmt.Println(tui.HelpStyle.Render("Tip: list the team's keys with `lsh ssh-keys list`; create one in this project with `lsh projects ssh-keys create`."))
			return nil
		}
		utils.Render(keys.GetData())
	}

	return nil
}
