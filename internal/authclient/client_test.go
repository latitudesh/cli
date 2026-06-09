package authclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	return New(srv.URL, "lsh-test/0.0.0", "2023-06-01"), srv.Close
}

func TestCreateSession(t *testing.T) {
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/cli_sessions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("missing Content-Type")
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"data":{"id":"sid","secret":"shh","user_code":"WDJB-MJHT","authorize_url":"https://x/y?session=sid","expires_at":"2026-01-01T00:00:00Z"}}`))
	})
	defer stop()

	got, err := c.CreateSession(context.Background(), CreateSessionRequest{ClientName: "lsh", ClientVersion: "1.0"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if got.ID != "sid" || got.Secret != "shh" || got.UserCode != "WDJB-MJHT" {
		t.Fatalf("unexpected session payload: %+v", got)
	}
}

func TestPollSessionPending(t *testing.T) {
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/cli_sessions/sid" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-CLI-Secret") != "shh" {
			t.Fatalf("missing X-CLI-Secret")
		}
		w.Write([]byte(`{"data":{"status":"pending"}}`))
	})
	defer stop()

	got, err := c.PollSession(context.Background(), "sid", "shh")
	if err != nil {
		t.Fatalf("PollSession: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("expected pending, got %s", got.Status)
	}
	if got.APIKey != nil {
		t.Fatalf("did not expect api_key on pending payload")
	}
}

func TestPollSessionApproved(t *testing.T) {
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"status":"approved","api_key":{"id":"k","token":"t","name":"lsh"},"team":{"id":"team_abc","name":"Acme","slug":"acme"},"user":{"id":"u","email":"u@example.com"}}}`))
	})
	defer stop()

	got, err := c.PollSession(context.Background(), "sid", "shh")
	if err != nil {
		t.Fatalf("PollSession: %v", err)
	}
	if got.Status != "approved" {
		t.Fatalf("expected approved, got %s", got.Status)
	}
	if got.APIKey == nil || got.APIKey.Token != "t" {
		t.Fatalf("missing api_key in approved payload: %+v", got)
	}
	if got.Team == nil || got.Team.Slug != "acme" {
		t.Fatalf("missing team: %+v", got.Team)
	}
}

func TestPollSessionGone(t *testing.T) {
	c, stop := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		w.Write([]byte(`{"data":{"status":"gone"}}`))
	})
	defer stop()

	_, err := c.PollSession(context.Background(), "sid", "shh")
	if err == nil {
		t.Fatal("expected HTTPError on 410")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusGone {
		t.Fatalf("expected 410, got %d", httpErr.StatusCode)
	}
}

func TestPollSessionRejectsEmptyArgs(t *testing.T) {
	c := New("http://example.invalid", "lsh-test/0.0.0", "2023-06-01")
	if _, err := c.PollSession(context.Background(), "", "shh"); err == nil {
		t.Fatal("expected error on empty id")
	}
	if _, err := c.PollSession(context.Background(), "sid", ""); err == nil {
		t.Fatal("expected error on empty secret")
	}
}

func TestRevokeAPIKey(t *testing.T) {
	called := false
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/auth/api_keys/key_xyz" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "ak_xxx" {
			t.Fatalf("expected Authorization header to carry the token directly, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer stop()

	if err := c.RevokeAPIKey(context.Background(), "ak_xxx", "key_xyz"); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	if !called {
		t.Fatal("expected API to be called")
	}
}

func TestGetUserProfile(t *testing.T) {
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/profile" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "ak_xxx" {
			t.Fatalf("expected Authorization to carry token directly")
		}
		w.Write([]byte(`{"data":{"attributes":{"id":"u","email":"u@example.com","team":{"id":"team_abc","name":"Acme","slug":"acme"}}}}`))
	})
	defer stop()

	got, err := c.GetUserProfile(context.Background(), "ak_xxx")
	if err != nil {
		t.Fatalf("GetUserProfile: %v", err)
	}
	if got.Email != "u@example.com" || got.Team.Slug != "acme" {
		t.Fatalf("unexpected profile payload: %+v", got)
	}
}
