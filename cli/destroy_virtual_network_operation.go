package cli

import (
	"github.com/latitudesh/lsh/client/virtual_networks"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

type DeleteVirtualNetworkOperation struct {
	PathParamFlags cmdflag.Flags
}

func makeOperationVirtualNetworksDestroyVirtualNetworkCmd() (*cobra.Command, error) {
	operation := DeleteVirtualNetworkOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

func (o *DeleteVirtualNetworkOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Delete a virtual network",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *DeleteVirtualNetworkOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Virtual Network ID",
			Description: "The virtual network ID",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DeleteVirtualNetworkOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DeleteVirtualNetworkOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := virtual_networks.NewDestroyVirtualNetworkParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.VirtualNetworks.DestroyVirtualNetwork(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
