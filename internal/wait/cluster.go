package wait

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

// ErrClusterFailed is returned when a cluster enters its Failed phase instead
// of reaching a target phase.
var ErrClusterFailed = errors.New("kubernetes cluster entered a failed phase")

// ForKubernetesClusterPhase polls GET /kubernetes_clusters/{id} until the
// cluster's phase is one of want (success), one of fail (ErrClusterFailed),
// o.Timeout elapses (ErrTimeout), or the user cancels (ErrCanceled). It returns
// the last phase observed.
//
// Transient API errors are swallowed and retried until the timeout, at which
// point the last error is attached to ErrTimeout so the failure is diagnosable.
func ForKubernetesClusterPhase(
	ctx context.Context,
	client *sdk.Latitudesh,
	clusterID string,
	want, fail []components.KubernetesClusterDataPhase,
	o Options,
	opts ...operations.Option,
) (components.KubernetesClusterDataPhase, error) {
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	var (
		last    components.KubernetesClusterDataPhase
		lastErr error
	)

	err := Poll(ctx, DefaultBackoff(), func(ctx context.Context) (bool, error) {
		resp, err := client.KubernetesClusters.GetKubernetesCluster(ctx, clusterID, opts...)
		if err != nil {
			if isTerminalAPIError(err) {
				return false, err
			}
			lastErr = err
			return false, nil
		}

		phase := clusterPhase(resp)
		if phase == nil {
			return false, nil
		}
		last = *phase

		if containsClusterPhase(fail, *phase) {
			return false, fmt.Errorf("%w: %q", ErrClusterFailed, *phase)
		}
		return containsClusterPhase(want, *phase), nil
	})

	if errors.Is(err, ErrTimeout) && lastErr != nil {
		return last, fmt.Errorf("%w (last API error: %v)", ErrTimeout, lastErr)
	}
	return last, err
}

// clusterPhase extracts the phase from a GetKubernetesCluster response,
// tolerating any nil link in the data → attributes → phase chain.
func clusterPhase(resp *operations.GetKubernetesClusterResponse) *components.KubernetesClusterDataPhase {
	if resp == nil || resp.KubernetesCluster == nil || resp.KubernetesCluster.Data == nil || resp.KubernetesCluster.Data.Attributes == nil {
		return nil
	}
	return resp.KubernetesCluster.Data.Attributes.Phase
}

func containsClusterPhase(set []components.KubernetesClusterDataPhase, p components.KubernetesClusterDataPhase) bool {
	for _, v := range set {
		if v == p {
			return true
		}
	}
	return false
}
