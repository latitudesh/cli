package cli

import (
	"context"
	"fmt"
	"os"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationBlockListCmd() (*cobra.Command, error) {
	operation := BlockListOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type BlockListOperation struct {
	QueryParamFlags cmdflag.Flags
}

func (o *BlockListOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "list",
		Short:  "List all block storages",
		Long:   "List all block storages for your team, optionally filtered by project",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *BlockListOperation) registerFlags(cmd *cobra.Command) {
	o.QueryParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	queryParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "project",
			Label:       "Project ID or Slug",
			Description: "Filter block storages by project ID or slug",
			Required:    false,
		},
	}

	o.QueryParamFlags.Register(queryParamsSchema)
}

func (o *BlockListOperation) preRun(cmd *cobra.Command, args []string) {
	o.QueryParamFlags.PreRun(cmd, args)
}

func (o *BlockListOperation) run(cmd *cobra.Command, args []string) error {
	// Get optional project filter
	project, _ := cmd.Flags().GetString("project")

	if dryRun {
		logDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	// Initialize the SDK client
	apiKey := viper.GetString("Authorization")
	if apiKey == "" {
		return fmt.Errorf("API key not found. Please run 'lsh login' first")
	}

	ctx := context.Background()
	client := latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity(apiKey),
	)

	// Create filter pointer if project is specified
	var filterProject *string
	if project != "" {
		filterProject = &project
	}

	// Call the API
	response, err := client.Storage.GetStorageBlocks(ctx, filterProject)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !debug {
		if response != nil && response.HTTPMeta.Response != nil {
			fmt.Fprintf(os.Stdout, "Block storages listed successfully (Status: %s)\n", response.HTTPMeta.Response.Status)
			fmt.Fprintf(os.Stdout, "\nNote: Use 'lsh block get --id <BLOCK_ID>' to see full details including connector information\n")
		}
	}

	return nil
}
