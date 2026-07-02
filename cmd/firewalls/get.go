package firewalls

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetFirewallOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single firewall by its ID.\n",
		RunE:    op.run,
		Short:   "Get a firewall",
		Example: `  lsh firewalls get fw_xxxxxxxx`,
		Use:     "get <id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type GetFirewallOperation struct{}

func (o *GetFirewallOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.Firewalls.Get(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.Firewall != nil && response.Firewall.Data != nil && !lsh.Debug {
		firewall := Firewall{FirewallData: *response.Firewall.Data}
		utils.Render(firewall.GetData())
	}

	return nil
}
