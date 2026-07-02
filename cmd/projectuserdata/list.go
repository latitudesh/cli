package projectuserdata

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/cmd/userdata"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListProjectUserDataOperation{}
	cmd := &cobra.Command{
		Long:  "List the user data entries scoped to a project.\n\nDefaults to the active project when --project is omitted.",
		RunE:  op.run,
		Short: "List project user data",
		Example: `  lsh projects user-data list
  lsh projects user-data list --project my-project`,
		Use:     "list",
		Aliases: []string{"ls"},
	}

	registerProjectFlag(cmd)

	return cmd
}

type ListProjectUserDataOperation struct{}

func (o *ListProjectUserDataOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	project := projectID(cmd)
	response, err := client.UserData.GetProjectUsersData(ctx, project, nil, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	list := userdata.UserDataList{}
	if response.UserData != nil {
		for i := range response.UserData.Data {
			list.Data = append(list.Data, &userdata.UserData{UserDataProperties: response.UserData.Data[i]})
		}
	}

	if !lsh.Debug {
		// A friendly empty state for the human view: an empty project is not an
		// error, and the entry the user is looking for may live at team level.
		// Structured formats (-o json/yaml/csv) still render the empty list.
		if len(list.Data) == 0 && renderer.ResolveFormat() == renderer.FormatTable {
			fmt.Printf("No user data entries in project %q.\n", project)
			fmt.Println(tui.HelpStyle.Render("Tip: list team-level user data with `lsh user-data list`; create one in this project with `lsh projects user-data create`."))
			return nil
		}
		utils.Render(list.GetData())
	}

	return nil
}
