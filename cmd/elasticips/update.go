package elasticips

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewUpdateCmd() *cobra.Command {
	op := UpdateElasticIPOperation{}
	cmd := &cobra.Command{
		Long:    "Move an Elastic IP to a different server.\n",
		RunE:    op.run,
		Short:   "Update an Elastic IP (reassign to another server)",
		Example: `  lsh elastic-ips update eip_xxxxxxxx --server sv_yyyyyyyy`,
		Use:     "update <id>",
		Args:    cobra.ExactArgs(1),
	}

	cmd.Flags().String("server", "", "Server ID to move the Elastic IP to")

	return cmd
}

type UpdateElasticIPOperation struct{}

// buildUpdateRequest validates flags and assembles the SDK request body.
func buildUpdateRequest(cmd *cobra.Command) (components.UpdateElasticIP, error) {
	server, _ := cmd.Flags().GetString("server")
	if server == "" {
		return components.UpdateElasticIP{}, fmt.Errorf("--server is required")
	}

	request := components.UpdateElasticIP{
		Data: components.UpdateElasticIPData{
			Type: components.UpdateElasticIPTypeElasticIps,
			Attributes: components.UpdateElasticIPAttributes{
				ServerID: server,
			},
		},
	}

	return request, nil
}

func (o *UpdateElasticIPOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	request, err := buildUpdateRequest(cmd)
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	response, err := client.ElasticIps.UpdateElasticIP(ctx, id, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.ElasticIP != nil && response.ElasticIP.Data != nil && !lsh.Debug {
		ip := ElasticIP{ElasticIPData: *response.ElasticIP.Data}
		utils.RenderStatic(ip.GetData())
	}

	return nil
}
