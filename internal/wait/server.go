package wait

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

// ErrFailedState is returned when a server reaches a state the caller declared
// as terminal-failure (e.g. failed_deployment) instead of its target state.
var ErrFailedState = errors.New("server entered a failed state")

// ForServerState polls GET /servers/{id} until the server's status is one of
// want (success), one of fail (returns ErrFailedState), o.Timeout elapses
// (ErrTimeout), or the user cancels (ErrCanceled). It returns the last status
// observed and invokes onStatus (when non-nil) on every successful poll.
//
// When requireTransition is true, a terminal state is only accepted after the
// server has first been observed in some other (transition) state. This guards
// operations that act on a server already sitting in a target state — e.g. a
// reinstall on a powered-on server, or a create whose POST response optimistically
// reports "on" — from returning before the operation has actually begun.
//
// Transient errors from the API (the platform occasionally 5xxs mid-provision)
// are not fatal: they are swallowed and retried until the timeout, at which
// point the last error is attached to ErrTimeout so the failure is diagnosable.
func ForServerState(
	ctx context.Context,
	client *sdk.Latitudesh,
	serverID string,
	want, fail []components.ServerDataStatus,
	requireTransition bool,
	o Options,
	onStatus func(components.ServerDataStatus),
	opts ...operations.Option,
) (components.ServerDataStatus, error) {
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}

	var (
		last           components.ServerDataStatus
		lastErr        error
		seenTransition bool
	)

	err := Poll(ctx, DefaultBackoff(), func(ctx context.Context) (bool, error) {
		resp, err := client.Servers.Get(ctx, serverID, nil, opts...)
		if err != nil {
			// 4xx (bad ID, revoked token, …) won't fix itself by waiting — fail
			// fast. 5xx / 429 / connection errors are transient: keep polling and
			// surface the last one only if we ultimately time out.
			if isTerminalAPIError(err) {
				return false, err
			}
			lastErr = err
			return false, nil
		}

		status := serverStatus(resp)
		if status == nil {
			return false, nil
		}
		last = *status
		if onStatus != nil {
			onStatus(*status)
		}

		done, transitioned, decideErr := decideServerState(*status, want, fail, requireTransition, seenTransition)
		seenTransition = transitioned
		if decideErr != nil {
			return false, decideErr
		}
		return done, nil
	})

	if errors.Is(err, ErrTimeout) && lastErr != nil {
		return last, fmt.Errorf("%w (last API error: %v)", ErrTimeout, lastErr)
	}
	return last, err
}

// decideServerState evaluates a single observed status against the target sets.
// It returns whether the wait is satisfied, the updated seenTransition flag, and
// a terminal error when the server reached a failure state.
//
// Failure states are reported immediately, even before a transition state was
// observed — only the success path is gated on requireTransition, so that an
// operation acting on a server already in a target state (e.g. a reinstall on a
// powered-on server) does not return before it actually begins.
func decideServerState(
	status components.ServerDataStatus,
	want, fail []components.ServerDataStatus,
	requireTransition, seenTransition bool,
) (done bool, transitioned bool, err error) {
	inWant := containsStatus(want, status)
	inFail := containsStatus(fail, status)
	if !inWant && !inFail {
		seenTransition = true
	}

	if inFail {
		return false, seenTransition, fmt.Errorf("%w: %q", ErrFailedState, status)
	}
	if inWant && (!requireTransition || seenTransition) {
		return true, seenTransition, nil
	}
	return false, seenTransition, nil
}

// isTerminalAPIError reports whether err is a client (4xx, except 429) API
// error — one that will not resolve by waiting and should abort the poll.
func isTerminalAPIError(err error) bool {
	var apiErr *components.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case 408, 425, 429:
		// Request Timeout / Too Early / Too Many Requests are transient.
		return false
	}
	return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
}

// serverStatus extracts the status from a GetServer response, tolerating any
// nil link in the data → attributes → status chain.
func serverStatus(resp *operations.GetServerResponse) *components.ServerDataStatus {
	if resp == nil || resp.Server == nil || resp.Server.Data == nil || resp.Server.Data.Attributes == nil {
		return nil
	}
	return resp.Server.Data.Attributes.Status
}

func containsStatus(set []components.ServerDataStatus, s components.ServerDataStatus) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}
