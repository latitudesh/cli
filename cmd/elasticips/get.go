package elasticips

import (
	"context"

	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	op := GetElasticIPOperation{}
	cmd := &cobra.Command{
		Long:    "Retrieve a single Elastic IP by its ID.\n",
		RunE:    op.run,
		Short:   "Get an Elastic IP",
		Example: `  lsh elastic-ips get eip_xxxxxxxx`,
		Use:     "get <id>",
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

type GetElasticIPOperation struct{}

func (o *GetElasticIPOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.ElasticIps.GetElasticIP(ctx, id, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.ElasticIP != nil && response.ElasticIP.Data != nil && !lsh.Debug {
		ip := ElasticIP{ElasticIPData: *response.ElasticIP.Data}
		utils.Render(ip.GetData())
	}

	return nil
}
