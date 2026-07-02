package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/browser"
	"github.com/latitudesh/lsh/internal/config"
)

func TestLoginWithTokenSavesProfile(t *testing.T) {
	withTempHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/profile":
			w.Write([]byte(`{"data":{"attributes":{"id":"u","email":"u@example.com"}}}`))
		case "/team":
			w.Write([]byte(`{"data":[{"id":"team_abc","attributes":{"name":"Acme","slug":"acme"}}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")
	if err := loginWithToken(context.Background(), client, "ak_xxx", ""); err != nil {
		t.Fatalf("loginWithToken: %v", err)
	}

	f, _ := config.Load()
	p, ok := f.Profiles["acme"]
	if !ok {
		t.Fatalf("expected profile 'acme', got %+v", f.Profiles)
	}
	if p.Authorization != "ak_xxx" || p.Email != "u@example.com" || p.TeamSlug != "acme" {
		t.Fatalf("unexpected profile: %+v", p)
	}
	if p.Source != config.SourceWithToken {
		t.Fatalf("expected with-token source, got %q", p.Source)
	}
}

func TestLoginViaBrowserSavesProfile(t *testing.T) {
	withTempHome(t)

	prevOpener := browser.Opener
	browser.Opener = func(string) error { return nil }
	t.Cleanup(func() { browser.Opener = prevOpener })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/auth/cli_sessions":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data":{"id":"sid","secret":"shh","authorize_url":"https://example/auth","user_code":"WXYZ","expires_at":"2030-01-01T00:00:00Z"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/auth/cli_sessions/sid":
			w.Write([]byte(`{"data":{"status":"approved","api_key":{"id":"k","token":"ak_browser","name":"lsh"},"team":{"id":"team_abc","name":"Acme","slug":"acme"},"user":{"id":"u","email":"u@example.com"}}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")
	if err := loginViaBrowser(context.Background(), client, ""); err != nil {
		t.Fatalf("loginViaBrowser: %v", err)
	}

	f, _ := config.Load()
	p, ok := f.Profiles["acme"]
	if !ok {
		t.Fatalf("expected profile 'acme', got %+v", f.Profiles)
	}
	if p.Authorization != "ak_browser" || p.KeyID != "k" || p.Source != config.SourceBrowser {
		t.Fatalf("unexpected profile: %+v", p)
	}
}
