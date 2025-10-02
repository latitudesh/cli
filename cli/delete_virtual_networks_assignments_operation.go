package cli

import (
	"github.com/latitudesh/lsh/client/virtual_network_assignments"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/cmdflag"
	"github.com/latitudesh/lsh/internal/utils"

	"github.com/spf13/cobra"
)

func makeOperationVirtualNetworkAssignmentsDeleteVirtualNetworksAssignmentsCmd() (*cobra.Command, error) {
	operation := DeleteVirtualNetworkAssignmentOperation{}

	cmd, err := operation.Register()
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

type DeleteVirtualNetworkAssignmentOperation struct {
	PathParamFlags cmdflag.Flags
}

func (o *DeleteVirtualNetworkAssignmentOperation) Register() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Unassign a server from a virtual network",
		RunE:   o.run,
		PreRun: o.preRun,
	}

	o.registerFlags(cmd)

	return cmd, nil
}

func (o *DeleteVirtualNetworkAssignmentOperation) PromptAttributes(attributes interface{}) {
}

func (o *DeleteVirtualNetworkAssignmentOperation) registerFlags(cmd *cobra.Command) {
	o.PathParamFlags = cmdflag.Flags{FlagSet: cmd.Flags()}

	schema := &cmdflag.FlagsSchema{
		&cmdflag.String{
			Name:        "id",
			Label:       "Virtual Network Assignment ID",
			Description: "Virtual Network Assignment ID.",
			Required:    true,
		},
	}

	o.PathParamFlags.Register(schema)
}

func (o *DeleteVirtualNetworkAssignmentOperation) preRun(cmd *cobra.Command, args []string) {
	o.PathParamFlags.PreRun(cmd, args)
}

func (o *DeleteVirtualNetworkAssignmentOperation) run(cmd *cobra.Command, args []string) error {
	appCli, err := makeClient(cmd, args)
	if err != nil {
		return err
	}

	params := virtual_network_assignments.NewDeleteVirtualNetworksAssignmentsParams()
	o.PathParamFlags.AssignValues(params)

	if lsh.DryRun {
		lsh.LogDebugf("dry-run flag specified. Skip sending request.")
		return nil
	}

	response, err := appCli.VirtualNetworkAssignments.DeleteVirtualNetworksAssignments(params, nil)
	if err != nil {
		utils.PrintError(err)
		return nil
	}

	if !lsh.Debug {
		response.Render()
	}
	return nil
}
