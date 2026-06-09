package cli

import (
	"github.com/spf13/cobra"
)

func makeOperationGroupVirtualNetworkAssignmentCmd() (*cobra.Command, error) {
	operationGroupVirtualNetworkAssignmentsCmd := &cobra.Command{
		Use:   "assignments",
		Short: "Manage virtual network assignments",
		Long:  "Create, list, and delete assignments between virtual networks and servers.",
	}

	operationAssignServerVirtualNetworkCmd, err := makeOperationVirtualNetworkAssignmentsAssignServerVirtualNetworkCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworkAssignmentsCmd.AddCommand(operationAssignServerVirtualNetworkCmd)

	operationDeleteVirtualNetworksAssignmentsCmd, err := makeOperationVirtualNetworkAssignmentsDeleteVirtualNetworksAssignmentsCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworkAssignmentsCmd.AddCommand(operationDeleteVirtualNetworksAssignmentsCmd)

	operationGetVirtualNetworksAssignmentsCmd, err := makeOperationVirtualNetworkAssignmentsGetVirtualNetworksAssignmentsCmd()
	if err != nil {
		return nil, err
	}
	operationGroupVirtualNetworkAssignmentsCmd.AddCommand(operationGetVirtualNetworksAssignmentsCmd)

	return operationGroupVirtualNetworkAssignmentsCmd, nil
}
