package firewalls

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cli"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	op := ListFirewallsOperation{}
	cmd := &cobra.Command{
		Long:  "List all firewalls in the team. Use --project to filter by project.\n",
		RunE:  op.run,
		Short: "List firewalls",
		Example: `  lsh firewalls list
  lsh firewalls list --project my-project
  lsh firewalls list -o json`,
		Use:         "list",
		Aliases:     []string{"ls"},
		Annotations: map[string]string{cli.ProjectOptionalAnnotation: "true"},
	}

	cmd.Flags().String("project", "", "Filter by project ID or slug")

	return cmd
}

type ListFirewallsOperation struct{}

func (o *ListFirewallsOperation) run(cmd *cobra.Command, args []string) error {
	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	var filterProject *string
	if cmd.Flags().Changed("project") {
		value, _ := cmd.Flags().GetString("project")
		filterProject = &value
	}

	response, err := client.Firewalls.List(ctx, filterProject, nil, nil, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	firewalls := Firewalls{}
	if response.Firewalls != nil {
		for i := range response.Firewalls.Data {
			firewalls.Data = append(firewalls.Data, &Firewall{FirewallData: response.Firewalls.Data[i]})
		}
	}

	if !lsh.Debug {
		utils.Render(firewalls.GetData())
	}

	return nil
}
