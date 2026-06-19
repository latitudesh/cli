package wait

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/latitudesh/latitudesh-go-sdk/models/components"
	"github.com/latitudesh/latitudesh-go-sdk/models/operations"
)

func TestBackoffNext(t *testing.T) {
	b := Backoff{Initial: 2 * time.Second, Max: 10 * time.Second, Factor: 2}

	cases := []struct {
		prev time.Duration
		want time.Duration
	}{
		{0, 2 * time.Second},               // first delay is Initial
		{2 * time.Second, 4 * time.Second}, // grows by Factor
		{4 * time.Second, 8 * time.Second},
		{8 * time.Second, 10 * time.Second}, // capped at Max
	}
	for _, c := range cases {
		if got := b.next(c.prev); got != c.want {
			t.Errorf("next(%v) = %v, want %v", c.prev, got, c.want)
		}
	}
}

func TestBackoffNextRespectsFloor(t *testing.T) {
	// A sub-second Initial must be lifted to the 1 req/s floor.
	b := Backoff{Initial: 100 * time.Millisecond, Max: 5 * time.Second, Factor: 1.5}
	if got := b.next(0); got < pollFloor {
		t.Errorf("next(0) = %v, want >= %v", got, pollFloor)
	}
}

func TestPollStopsWhenDone(t *testing.T) {
	b := Backoff{Initial: time.Millisecond, Max: time.Millisecond, Factor: 1}
	calls := 0
	err := Poll(context.Background(), b, func(context.Context) (bool, error) {
		calls++
		return calls == 3, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("probe called %d times, want 3", calls)
	}
}

func TestPollProbesImmediately(t *testing.T) {
	// First probe must run with no initial delay.
	b := Backoff{Initial: time.Hour, Max: time.Hour, Factor: 1}
	err := Poll(context.Background(), b, func(context.Context) (bool, error) {
		return true, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPollPropagatesProbeError(t *testing.T) {
	sentinel := errors.New("boom")
	b := Backoff{Initial: time.Millisecond, Max: time.Millisecond, Factor: 1}
	err := Poll(context.Background(), b, func(context.Context) (bool, error) {
		return false, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want %v", err, sentinel)
	}
}

func TestPollTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	b := Backoff{Initial: 5 * time.Millisecond, Max: 5 * time.Millisecond, Factor: 1}
	err := Poll(ctx, b, func(context.Context) (bool, error) {
		return false, nil // never done
	})
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("got %v, want ErrTimeout", err)
	}
}

func TestPollCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := Backoff{Initial: 5 * time.Millisecond, Max: 5 * time.Millisecond, Factor: 1}
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := Poll(ctx, b, func(context.Context) (bool, error) {
		return false, nil
	})
	if !errors.Is(err, ErrCanceled) {
		t.Errorf("got %v, want ErrCanceled", err)
	}
}

func TestServerStatusNilSafe(t *testing.T) {
	if serverStatus(nil) != nil {
		t.Error("serverStatus(nil) should be nil")
	}
	if serverStatus(&operations.GetServerResponse{}) != nil {
		t.Error("serverStatus with nil Server should be nil")
	}
	on := components.ServerDataStatusOn
	resp := &operations.GetServerResponse{
		Server: &components.Server{
			Data: &components.ServerData{
				Attributes: &components.ServerDataAttributes{Status: &on},
			},
		},
	}
	got := serverStatus(resp)
	if got == nil || *got != components.ServerDataStatusOn {
		t.Errorf("serverStatus = %v, want on", got)
	}
}

func TestContainsStatus(t *testing.T) {
	set := []components.ServerDataStatus{
		components.ServerDataStatusOn,
		components.ServerDataStatusDeploying,
	}
	if !containsStatus(set, components.ServerDataStatusOn) {
		t.Error("expected on to be in set")
	}
	if containsStatus(set, components.ServerDataStatusOff) {
		t.Error("did not expect off to be in set")
	}
}

func TestDecideServerState(t *testing.T) {
	want := []components.ServerDataStatus{components.ServerDataStatusOn, components.ServerDataStatusOff}
	fail := []components.ServerDataStatus{components.ServerDataStatusFailedDeployment}

	t.Run("want hit without requireTransition is done", func(t *testing.T) {
		done, _, err := decideServerState(components.ServerDataStatusOn, want, fail, false, false)
		if err != nil || !done {
			t.Fatalf("done=%v err=%v, want done=true err=nil", done, err)
		}
	})

	t.Run("want hit is gated until a transition is seen", func(t *testing.T) {
		done, transitioned, err := decideServerState(components.ServerDataStatusOn, want, fail, true, false)
		if err != nil || done {
			t.Fatalf("done=%v err=%v, want done=false err=nil (gated)", done, err)
		}
		if transitioned {
			t.Error("a target state should not count as a transition")
		}
	})

	t.Run("transition state flips seenTransition then want succeeds", func(t *testing.T) {
		_, transitioned, _ := decideServerState(components.ServerDataStatusDeploying, want, fail, true, false)
		if !transitioned {
			t.Fatal("deploying should set seenTransition")
		}
		done, _, err := decideServerState(components.ServerDataStatusOn, want, fail, true, transitioned)
		if err != nil || !done {
			t.Fatalf("done=%v err=%v, want done=true after transition", done, err)
		}
	})

	t.Run("fail state reported immediately even without transition (H1 regression)", func(t *testing.T) {
		done, _, err := decideServerState(components.ServerDataStatusFailedDeployment, want, fail, true, false)
		if done {
			t.Error("fail state must not be 'done'")
		}
		if !errors.Is(err, ErrFailedState) {
			t.Errorf("got %v, want ErrFailedState", err)
		}
	})
}

func TestIsTerminalAPIError(t *testing.T) {
	cases := []struct {
		code int
		want bool
	}{
		{404, true},
		{401, true},
		{403, true},
		{422, true},
		{408, false}, // request timeout: transient
		{425, false}, // too early: transient
		{429, false}, // rate-limited: transient
		{500, false},
		{503, false},
	}
	for _, c := range cases {
		err := components.NewAPIError("x", c.code, "", nil)
		if got := isTerminalAPIError(err); got != c.want {
			t.Errorf("status %d: got %v, want %v", c.code, got, c.want)
		}
	}
	if isTerminalAPIError(errors.New("plain")) {
		t.Error("a non-API error should not be terminal")
	}
}
