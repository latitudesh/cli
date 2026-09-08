package wait

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

// ForVirtualMachineState polls GET /virtual_machines/{id} until the VM's status
// is one of want (success), o.Timeout elapses (ErrTimeout), or the user cancels
// (ErrCanceled). It returns the last status observed.
//
// The virtual machine schema does not expose a distinct failure status, so
// there is no fail set: a stuck provision surfaces as an ErrTimeout instead.
// Transient API errors are swallowed and retried until the timeout, at which
// point the last error is attached to ErrTimeout so the failure is diagnosable.
func ForVirtualMachineState(
	ctx context.Context,
	client *sdk.Latitudesh,
	vmID string,
	want []VirtualMachineStatus,
	o Options,
	opts ...operations.Option,
) (VirtualMachineStatus, error) {
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	var (
		last    VirtualMachineStatus
		lastErr error
	)

	err := Poll(ctx, DefaultBackoff(), func(ctx context.Context) (bool, error) {
		resp, err := client.VirtualMachines.Get(ctx, vmID, nil, opts...)
		if err != nil {
			if isTerminalAPIError(err) {
				return false, err
			}
			lastErr = err
			return false, nil
		}

		status := virtualMachineStatus(resp)
		if status == nil {
			return false, nil
		}
		last = *status
		return containsVMStatus(want, *status), nil
	})

	if errors.Is(err, ErrTimeout) && lastErr != nil {
		return last, fmt.Errorf("%w (last API error: %v)", ErrTimeout, lastErr)
	}
	return last, err
}

// virtualMachineStatus extracts the status from a ShowVirtualMachine response,
// tolerating any nil link in the data → attributes → status chain.
func virtualMachineStatus(resp *operations.ShowVirtualMachineResponse) *VirtualMachineStatus {
	if resp == nil || resp.VirtualMachine == nil || resp.VirtualMachine.Data == nil || resp.VirtualMachine.Data.Attributes == nil {
		return nil
	}
	if resp.VirtualMachine.Data.Attributes.Status == nil {
		return nil
	}
	st := VirtualMachineStatus(*resp.VirtualMachine.Data.Attributes.Status)
	return &st
}

func containsVMStatus(set []VirtualMachineStatus, s VirtualMachineStatus) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}
