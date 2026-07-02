package elasticips

import (
	"context"
	"fmt"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/latitudesh/lsh/internal/utils"
	cobra "github.com/spf13/cobra"
)

func NewCreateCmd() *cobra.Command {
	op := CreateElasticIPOperation{}
	cmd := &cobra.Command{
		Long: "Allocate a new Elastic IP and assign it to a server.\n\n" +
			"The API allocates the IP in the region of the target server and assigns\n" +
			"it to that server; both --project and --server are required. The IP is\n" +
			"provisioned asynchronously and starts in the 'configuring' status.\n",
		RunE:    op.run,
		Short:   "Create (allocate) an Elastic IP",
		Example: `  lsh elastic-ips create --project my-project --server sv_xxxxxxxx`,
		Use:     "create",
	}

	cmd.Flags().String("project", "", "Project ID or slug to allocate the Elastic IP in")
	cmd.Flags().String("server", "", "Server ID to assign the Elastic IP to")

	return cmd
}

type CreateElasticIPOperation struct{}

// buildCreateRequest validates flags and assembles the SDK request body. Split
// out so tests can exercise it without a network call.
func buildCreateRequest(cmd *cobra.Command) (components.CreateElasticIP, error) {
	project, _ := cmd.Flags().GetString("project")
	server, _ := cmd.Flags().GetString("server")

	if project == "" {
		return components.CreateElasticIP{}, fmt.Errorf("--project is required")
	}
	if server == "" {
		return components.CreateElasticIP{}, fmt.Errorf("--server is required")
	}

	request := components.CreateElasticIP{
		Data: components.CreateElasticIPData{
			Type: components.CreateElasticIPTypeElasticIps,
			Attributes: components.CreateElasticIPAttributes{
				ProjectID: project,
				ServerID:  server,
			},
		},
	}

	return request, nil
}

func (o *CreateElasticIPOperation) run(cmd *cobra.Command, args []string) error {
	request, err := buildCreateRequest(cmd)
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

	response, err := client.ElasticIps.CreateElasticIP(ctx, request, operations.WithRetries(lsh.RetryConfig()))
	if err != nil {
		utils.PrintError(err)
		return err
	}

	if response.ElasticIP != nil && response.ElasticIP.Data != nil && !lsh.Debug {
		fmt.Println(tui.SuccessStyle.Render("✓ Elastic IP allocation requested (provisioning asynchronously)."))
		ip := ElasticIP{ElasticIPData: *response.ElasticIP.Data}
		utils.RenderStatic(ip.GetData())
	}

	return nil
}
