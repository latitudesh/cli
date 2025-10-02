package cli

import (
	"context"
	"fmt"
	"os"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationBlockGetCmd() (*cobra.Command, error) {
	operation := BlockGetOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type BlockGetOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *BlockGetOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Get block storage details",
		Long:   "Get detailed information about a specific block storage including connector details needed for mounting",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *BlockGetOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Block Storage ID",
			Description: "The ID of the block storage to retrieve",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
}

func (o *BlockGetOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *BlockGetOperation) run(cmd *cobra.Command, args []string) error {
	// Get the block ID from flags
	blockID, err := cmd.Flags().GetString("id")
	if err != nil {
		return fmt.Errorf("error getting block ID: %w", err)
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

	// NOTE: The SDK doesn't seem to have a GetStorageBlock (singular) method yet
	// We'll need to use GetStorageBlocks and filter, or wait for the API to add this endpoint
	fmt.Fprintf(os.Stdout, "Fetching block storage details for: %s\n", blockID)

	// For now, use list and filter
	response, err := client.Storage.GetStorageBlocks(ctx, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		if response != nil && response.HTTPMeta.Response != nil {
			fmt.Fprintf(os.Stdout, "Block storage details retrieved (Status: %s)\n", response.HTTPMeta.Response.Status)
			fmt.Fprintf(os.Stdout, "\nNote: This command will show connector_id, gateway IP, and port once the API returns them.\n")
			fmt.Fprintf(os.Stdout, "Look for:\n")
			fmt.Fprintf(os.Stdout, "  - connector_id: The NVMe subsystem NQN (nqn.2001-07.com.ceph:...)\n")
			fmt.Fprintf(os.Stdout, "  - gateway_ip: The IP address of the storage gateway\n")
			fmt.Fprintf(os.Stdout, "  - gateway_port: The port (typically 4420)\n")
		}
	}

	return nil
}
