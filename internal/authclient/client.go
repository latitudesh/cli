// Package authclient is a minimal HTTP client for the CLI session and
// profile endpoints that lsh uses during login/logout. These endpoints
// are not in the generated go-swagger client, so we keep a small,
// dependency-free wrapper here.
package authclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
)

// Client talks to the Latitude API for auth-related endpoints.
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

// New builds a Client. baseURL should include scheme and host (e.g.
// "https://api.latitude.sh").
func New(baseURL, userAgent string) *Client {
	return &Client{
		baseURL:    baseURL,
		userAgent:  userAgent,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// CreateSessionRequest is the body of POST /auth/cli_sessions.
type CreateSessionRequest struct {
	ClientName    string `json:"client_name,omitempty"`
	ClientVersion string `json:"client_version,omitempty"`
}

// Session is the public payload returned by the API on create and on
// the secret-gated poll (with the credential fields populated).
type Session struct {
	ID           string  `json:"id"`
	Secret       string  `json:"secret,omitempty"`
	UserCode     string  `json:"user_code,omitempty"`
	AuthorizeURL string  `json:"authorize_url,omitempty"`
	ExpiresAt    string  `json:"expires_at,omitempty"`
	Status       string  `json:"status,omitempty"`
	APIKey       *APIKey `json:"api_key,omitempty"`
	Team         *Team   `json:"team,omitempty"`
	User         *User   `json:"user,omitempty"`
}

type APIKey struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Name  string `json:"name"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// HTTPError is returned when the API responds with a non-2xx status.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("api error: status=%d body=%s", e.StatusCode, e.Body)
}

// CreateSession opens a new CLI login session. Unauthenticated.
func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	var session struct {
		Data Session `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/auth/cli_sessions", nil, req, &session); err != nil {
		return nil, err
	}
	return &session.Data, nil
}

// PollSession reads the session with the secret. Returns the Session
// (with credential fields when status=approved) or HTTPError. Callers
// treat 410 (gone) and 404 (not found) as terminal.
func (c *Client) PollSession(ctx context.Context, id, secret string) (*Session, error) {
	if id == "" {
		return nil, errors.New("authclient: empty session id")
	}
	if secret == "" {
		return nil, errors.New("authclient: empty secret")
	}
	headers := map[string]string{"X-CLI-Secret": secret}
	var resp struct {
		Data Session `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/auth/cli_sessions/"+id, headers, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// RevokeAPIKey deletes an API key by id. Used by `lsh auth logout` on
// sessions created via the browser flow.
func (c *Client) RevokeAPIKey(ctx context.Context, token, keyID string) error {
	if keyID == "" {
		return errors.New("authclient: empty key id")
	}
	headers := map[string]string{"Authorization": token}
	return c.do(ctx, http.MethodDelete, "/auth/api_keys/"+keyID, headers, nil, nil)
}

// UserProfile is the subset of GET /user/profile that lsh uses to
// validate a token and to populate config after a --with-token login.
type UserProfile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Team  Team   `json:"team"`
}

// GetUserProfile validates the token and returns user/team context.
// Used by `lsh login --with-token <T>`. Note: the Rails resource does
// not return the team in this payload; use GetCurrentTeam to fetch it.
func (c *Client) GetUserProfile(ctx context.Context, token string) (*UserProfile, error) {
	headers := map[string]string{"Authorization": token}
	var resp struct {
		Data struct {
			Attributes UserProfile `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/user/profile", headers, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data.Attributes, nil
}

// Project is the subset of fields the interactive project picker needs.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ListProjects returns the projects accessible to the token's team.
// Used by the interactive project picker when a command needs a
// project but the user did not pass --project.
func (c *Client) ListProjects(ctx context.Context, token string) ([]Project, error) {
	headers := map[string]string{"Authorization": token}
	var resp struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/projects", headers, nil, &resp); err != nil {
		return nil, err
	}
	projects := make([]Project, 0, len(resp.Data))
	for _, p := range resp.Data {
		projects = append(projects, Project{ID: p.ID, Name: p.Attributes.Name, Slug: p.Attributes.Slug})
	}
	return projects, nil
}

// GetCurrentTeam returns the team bound to the token's membership.
// GET /team is server-side scoped to current_user_membership, so the
// returned list contains exactly one entry for a valid token.
func (c *Client) GetCurrentTeam(ctx context.Context, token string) (*Team, error) {
	headers := map[string]string{"Authorization": token}
	var resp struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/team", headers, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	first := resp.Data[0]
	return &Team{ID: first.ID, Name: first.Attributes.Name, Slug: first.Attributes.Slug}, nil
}

func (c *Client) do(ctx context.Context, method, path string, headers map[string]string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("authclient: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("authclient: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authclient: do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("authclient: decode response: %w", err)
	}
	return nil
}
