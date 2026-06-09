package authclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

func TestSendsAPIVersionHeader(t *testing.T) {
	var gotVersion string
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("API-Version")
		w.Write([]byte(`{"data":{"status":"pending"}}`))
	})
	defer stop()

	if _, err := c.PollSession(context.Background(), "sid", "shh"); err != nil {
		t.Fatalf("PollSession: %v", err)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("expected API-Version header, got %q", gotVersion)
	}
}

func TestListProjectsPaginates(t *testing.T) {
	all := []struct{ id, name, slug string }{
		{"p1", "One", "one"}, {"p2", "Two", "two"}, {"p3", "Three", "three"},
	}
	const perPage = 2 // server caps below the client's requested page size

	var requestedPages []string
	c, stop := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requestedPages = append(requestedPages, r.URL.Query().Get("page[number]"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page[number]"))
		if page < 1 {
			page = 1
		}
		var items []string
		for i := (page - 1) * perPage; i < page*perPage && i < len(all); i++ {
			p := all[i]
			items = append(items, fmt.Sprintf(`{"id":%q,"attributes":{"name":%q,"slug":%q}}`, p.id, p.name, p.slug))
		}
		fmt.Fprintf(w, `{"data":[%s],"meta":{"stats":{"total":{"count":%d}}}}`, strings.Join(items, ","), len(all))
	})
	defer stop()

	got, err := c.ListProjects(context.Background(), "ak_xxx")
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != len(all) {
		t.Fatalf("expected %d projects across pages, got %d", len(all), len(got))
	}
	if got[2].Slug != "three" {
		t.Fatalf("last paged project missing: %+v", got)
	}
	if len(requestedPages) < 2 {
		t.Fatalf("expected the client to page through results, requested pages: %v", requestedPages)
	}
}
