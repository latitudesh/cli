package cli

import (
	"context"
	"fmt"
	"os"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationVolumeDeleteCmd() (*cobra.Command, error) {
	operation := VolumeDeleteOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type VolumeDeleteOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *VolumeDeleteOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "delete",
		Short:  "Delete a volume storage",
		Long:   "Delete a volume storage by ID. Warning: This action cannot be undone!",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *VolumeDeleteOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Volume Storage ID",
			Description: "The ID of the volume storage to delete",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
}

func (o *VolumeDeleteOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *VolumeDeleteOperation) run(cmd *cobra.Command, args []string) error {
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

	// Confirm deletion
	fmt.Fprintf(os.Stdout, "⚠️  Warning: You are about to delete volume storage: %s\n", volumeID)
	fmt.Fprintf(os.Stdout, "This action cannot be undone. All data will be lost.\n\n")
	fmt.Fprintf(os.Stdout, "Type 'yes' to confirm: ")

	var confirmation string
	fmt.Scanln(&confirmation)

	if confirmation != "yes" {
		fmt.Fprintf(os.Stdout, "Deletion cancelled.\n")
		return nil
	}

	// Call the API
	response, err := client.Storage.DeleteStorageVolumes(ctx, volumeID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting volume storage: %v\n", err)
		return err
	}

	if !lsh.Debug {
		if response != nil && response.HTTPMeta.Response != nil {
			fmt.Fprintf(os.Stdout, "✅ Volume storage deleted successfully (Status: %s)\n", response.HTTPMeta.Response.Status)
		}
	}

	return nil
}
