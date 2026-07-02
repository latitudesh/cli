package elasticips

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
	op := DeleteElasticIPOperation{}
	cmd := &cobra.Command{
		Long:    "Release an Elastic IP by its ID.\n",
		RunE:    op.run,
		Short:   "Delete (release) an Elastic IP",
		Example: `  lsh elastic-ips delete eip_xxxxxxxx`,
		Use:     "delete <id>",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rm"},
	}

	return cmd
}

type DeleteElasticIPOperation struct{}

func (o *DeleteElasticIPOperation) run(cmd *cobra.Command, args []string) error {
	id := args[0]

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	client := lsh.NewClient()
	ctx := context.Background()

	resp, err := client.ElasticIps.DeleteElasticIP(ctx, id, operations.WithRetries(lsh.RetryConfig()))
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
			fmt.Printf("\nElastic IP released successfully!\n")
		} else {
			fmt.Printf("Warning: Unexpected status code: %d\n", status)
		}
	}

	return nil
}
