package virtualmachines

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
	"github.com/latitudesh/lsh/cmd/lsh"
	"github.com/latitudesh/lsh/internal/utils"
	"github.com/latitudesh/lsh/internal/wait"
	"github.com/spf13/cobra"
)

// waitForVirtualMachine blocks until the VM reaches its Running state, times
// out, or the user cancels — but only when --wait was passed. Progress and the
// final state are written to stderr/stdout so they never corrupt structured
// (-o json) output.
func waitForVirtualMachine(cmd *cobra.Command, vmID string) error {
	o := wait.OptionsFrom(cmd)
	if !o.Enabled {
		if cmd.Flags().Changed("timeout") {
			fmt.Fprintln(os.Stderr, "warning: --timeout has no effect without --wait")
		}
		return nil
	}
	if vmID == "" {
		fmt.Fprintln(os.Stderr, "warning: --wait ignored: could not determine the virtual machine ID")
		return nil
	}

	client := lsh.NewClient()
	ctx, cancel := wait.SignalContext(context.Background())
	defer cancel()

	fmt.Fprintf(os.Stderr, "Waiting for virtual machine %s to finish provisioning… (Ctrl+C to stop)\n", vmID)

	want := []components.VirtualMachineAttributesStatus{components.VirtualMachineAttributesStatusRunning}
	status, err := wait.ForVirtualMachineState(ctx, client, vmID, want, o, operations.WithRetries(lsh.RetryConfig()))
	switch {
	case errors.Is(err, wait.ErrCanceled):
		return fmt.Errorf("wait canceled (last status: %s)", status)
	case errors.Is(err, wait.ErrTimeout):
		return fmt.Errorf("timed out waiting for virtual machine %s (last status: %s)", vmID, status)
	case err != nil:
		return err
	}

	fmt.Fprintf(os.Stderr, "Virtual machine %s is now %q\n", vmID, status)

	// The create response is an optimistic snapshot; re-fetch to render the
	// real, final state. The wait itself already succeeded, so a failed
	// re-fetch only degrades the display — surface it on stderr.
	if !lsh.Debug {
		resp, err := client.VirtualMachines.Get(ctx, vmID, operations.WithRetries(lsh.RetryConfig()))
		if err == nil && resp.VirtualMachine != nil && resp.VirtualMachine.Data != nil {
			vm := VirtualMachine{VirtualMachineAttributes: *resp.VirtualMachine.Data}
			utils.RenderStatic(vm.GetData())
		} else {
			fmt.Fprintf(os.Stderr, "warning: could not fetch the final virtual machine state for display: %v\n", err)
		}
	}
	return nil
}
