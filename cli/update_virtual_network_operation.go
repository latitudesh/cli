package cli

import (
	"github.com/latitudesh/lsh/client/virtual_networks"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationVirtualNetworksUpdateVirtualNetworkCmd() (*cobra.Command, error) {
	operation := UpdateVirtualNetworkOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type UpdateVirtualNetworkOperation struct {
	PathParamFlags      cmdflag.Flags
	BodyAttributesFlags cmdflag.Flags
}

func (o *UpdateVirtualNetworkOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update a virtual network",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *UpdateVirtualNetworkOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	pathParamsSchema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Virtual Network ID",
			Description: "Virtual Network ID",
			Required:    true,
		},
	}

	bodyFlagsSchema := &cmdflag.FlagsSchema{
		&cmdflag.StringSlice{
			Name:        "tags",
			Label:       "Tags",
			Description: "Tags",
			Required:    false,
		},
		&cmdflag.String{
			Name:        "description",
			Label:       "Description",
			Description: "Description",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(pathParamsSchema)
	o.BodyAttributesFlags.Register(bodyFlagsSchema)
}

func (o *UpdateVirtualNetworkOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *UpdateVirtualNetworkOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := virtual_networks.NewUpdateVirtualNetworkParams()
	o.PathParamFlags.AssignValues(params)
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)
	params.Body.Data.ID = params.VirtualNetworkID

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.VirtualNetworks.UpdateVirtualNetwork(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
