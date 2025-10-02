package cli

import (
	"github.com/latitudesh/lsh/client/virtual_network_assignments"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationVirtualNetworkAssignmentsAssignServerVirtualNetworkCmd() (*cobra.Command, error) {
	operation := CreateVirtualNetworkAssignmentOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type CreateVirtualNetworkAssignmentOperation struct {
	BodyAttributesFlags cmdflag.Flags
}

func (o *CreateVirtualNetworkAssignmentOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "create",
		Short:  "Assign a server to a virtual network",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *CreateVirtualNetworkAssignmentOperation) registerFlags(cmd *cobra.Command) {
	o.BodyAttributesFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:           "server_id",
			Label:          "Server ID",
			CustomFlagName: "server",
			Description:    "Server ID",
			Required:       true,
		},
		&cmdflag.String{
			Name:           "virtual_network_id",
			Label:          "Virtual Network ID",
			CustomFlagName: "virtual_network",
			Description:    "Virtual Network ID",
			Required:       true,
		},
	}

	o.BodyAttributesFlags.Register(schema)
}

func (o *CreateVirtualNetworkAssignmentOperation) preRun(cmd *cobra.Command, args []string) {
	o.BodyAttributesFlags.PreRun(cmd, args)
}

func (o *CreateVirtualNetworkAssignmentOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := virtual_network_assignments.NewAssignServerVirtualNetworkParams()
	o.BodyAttributesFlags.AssignValues(params.Body.Data.Attributes)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.VirtualNetworkAssignments.AssignServerVirtualNetwork(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		utils.Render(response.GetData())
	}
	return nil
}
