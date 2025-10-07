package cli

import (
	"context"
	"fmt"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/output"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationVolumeListCmd() (*cobra.Command, error) {
	operation := VolumeListOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type VolumeListOperation struct {
	QueryParamFlags cmdflag.Flags
}

func (o *VolumeListOperation) Register() (*cobra.Command, error) {
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

func (o *VolumeListOperation) registerFlags(cmd *cobra.Command) {
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

func (o *VolumeListOperation) preRun(cmd *cobra.Command, args []string) {
	o.QueryParamFlags.PreRun(cmd, args)
}

func (o *VolumeListOperation) run(cmd *cobra.Command, args []string) error {
	// Get optional project filter
	project, _ := cmd.Flags().GetString("project")

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
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
	response, err := client.Storage.GetStorageVolumes(ctx, filterProject)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		if response.Object != nil && response.Object.Data != nil {
			// Display volumes as JSON
			output.RenderAsJSON(response.Object.Data)
		}
	}

	return nil
}
