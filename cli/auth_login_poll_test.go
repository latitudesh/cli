package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/latitudesh/lsh/internal/authclient"
)

// approvalGatedServer returns 401 until the request carries the matching
// approval code, mirroring the API's APPROVAL_CODE_MISMATCH gate.
func approvalGatedServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CLI-Approval-Code") != "the-code" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"errors":[{"code":"APPROVAL_CODE_MISMATCH"}]}`))
			return
		}
		w.Write([]byte(approvedBrowserProfile))
	}))
}

func TestPollUntilApprovedDelayedCallback(t *testing.T) {
	srv := approvalGatedServer(t)
	defer srv.Close()
	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")

	codeCh := make(chan string, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		codeCh <- "the-code"
	}()

	session, err := pollUntilApproved(context.Background(), client, "sid", "shh", codeCh, nil)
	if err != nil {
		t.Fatalf("pollUntilApproved: %v", err)
	}
	if session.APIKey == nil || session.APIKey.Token != "ak_browser" {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestPollUntilApprovedCancellationIsPrompt(t *testing.T) {
	srv := approvalGatedServer(t)
	defer srv.Close()
	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")

	ctx, cancel := context.WithCancel(context.Background())
	codeCh := make(chan string) // never delivers
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := pollUntilApproved(ctx, client, "sid", "shh", codeCh, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("cancellation was not prompt: waited %s", elapsed)
	}
}

func TestPollUntilApprovedListenerError(t *testing.T) {
	srv := approvalGatedServer(t)
	defer srv.Close()
	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")

	codeCh := make(chan string) // never delivers
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		errCh <- errors.New("serve failed")
	}()

	start := time.Now()
	_, err := pollUntilApproved(context.Background(), client, "sid", "shh", codeCh, errCh)
	if err == nil {
		t.Fatal("expected a listener error, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("listener error was not reported promptly: waited %s", elapsed)
	}
}
