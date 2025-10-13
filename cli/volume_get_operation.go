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

func makeOperationVolumeGetCmd() (*cobra.Command, error) {
	operation := VolumeGetOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type VolumeGetOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *VolumeGetOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Get volume storage details",
		Long:   "Get detailed information about a specific volume storage including connector details needed for mounting",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *VolumeGetOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Volume Storage ID",
			Description: "The ID of the volume storage to retrieve",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
}

func (o *VolumeGetOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *VolumeGetOperation) run(cmd *cobra.Command, args []string) error {
	// Get the volume ID from flags
	volumeID, err := cmd.Flags().GetString("id")
	if err != nil {
		return fmt.Errorf("error getting volume ID: %w", err)
	}

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

	// NOTE: The SDK doesn't seem to have a GetStorageVolume (singular) method yet
	// For now, use list and filter by ID
	response, err := client.Storage.GetStorageVolumes(ctx, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	// Filter the response to find the matching volume
	if response != nil && response.Object != nil && response.Object.Data != nil {
		for _, volume := range response.Object.Data {
			if volume.ID != nil && *volume.ID == volumeID {
				if !lsh.Debug {
					// Display single volume as JSON
					output.RenderAsJSON(volume)
				}
				return nil
			}
		}
		// Volume not found
		return fmt.Errorf("volume with ID '%s' not found", volumeID)
	}

	return fmt.Errorf("no data returned from API")
}
