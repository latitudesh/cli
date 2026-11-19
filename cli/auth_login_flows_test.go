package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

const approvedBrowserProfile = `{"data":{"status":"approved","api_key":{"id":"k","token":"ak_browser","name":"lsh"},"team":{"id":"team_abc","name":"Acme","slug":"acme"},"user":{"id":"u","email":"u@example.com"}}}`

func forceHeadless(t *testing.T, headless bool) {
	t.Helper()
	prev := looksHeadless
	looksHeadless = func() bool { return headless }
	t.Cleanup(func() { looksHeadless = prev })
}

func assertBrowserProfileSaved(t *testing.T) {
	t.Helper()
	f, _ := config.Load()
	p, ok := f.Profiles["acme"]
	if !ok {
		t.Fatalf("expected profile 'acme', got %+v", f.Profiles)
	}
	if p.Authorization != "ak_browser" || p.KeyID != "k" || p.Source != config.SourceBrowser {
		t.Fatalf("unexpected profile: %+v", p)
	}
}

func TestLoginViaBrowserLoopback(t *testing.T) {
	withTempHome(t)
	forceHeadless(t, false)

	prevOpener := browser.Opener
	browser.Opener = func(string) error { return nil }
	t.Cleanup(func() { browser.Opener = prevOpener })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/auth/cli_sessions":
			var body struct {
				CallbackURL string `json:"callback_url"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			if body.CallbackURL == "" {
				t.Errorf("expected a loopback callback_url, got none")
			} else {
				// Stand in for the dashboard redirecting to the loopback callback.
				go http.Get(body.CallbackURL + "&approval_code=the-code")
			}
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"data":{"id":"sid","secret":"shh","authorize_url":"https://example/auth","user_code":"WXYZ","expires_at":"2030-01-01T00:00:00Z"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/auth/cli_sessions/sid":
			// The credential is released only once the approval code is presented.
			if r.Header.Get("X-CLI-Approval-Code") != "the-code" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"errors":[{"code":"APPROVAL_CODE_MISMATCH"}]}`))
				return
			}
			w.Write([]byte(approvedBrowserProfile))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")
	if err := loginViaBrowser(context.Background(), client, ""); err != nil {
		t.Fatalf("loginViaBrowser: %v", err)
	}
	assertBrowserProfileSaved(t)
}

func TestLoginViaBrowserHeadless(t *testing.T) {
	withTempHome(t)
	forceHeadless(t, true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("headless login must not call the API, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client := authclient.New(srv.URL, "lsh-test", "2023-06-01")
	err := loginViaBrowser(context.Background(), client, "")
	if !errors.Is(err, errHeadlessLogin) {
		t.Fatalf("expected errHeadlessLogin, got %v", err)
	}
}
