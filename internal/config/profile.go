package config

// Profile is one (team, api_key) binding stored locally.
// Token sources: "browser" (created by `lsh login` via cli_sessions) or
// "with-token" (created by `lsh login --with-token <T>`).
type Profile struct {
	Authorization string `json:"authorization"`
	KeyID         string `json:"key_id,omitempty"`
	KeyName       string `json:"key_name,omitempty"`
	TeamID        string `json:"team_id,omitempty"`
	TeamName      string `json:"team_name,omitempty"`
	TeamSlug      string `json:"team_slug,omitempty"`
	Email         string `json:"email,omitempty"`
	Source        string `json:"source,omitempty"`
	APIVersion    string `json:"api_version,omitempty"`
}

// SourceBrowser is set on profiles created via the browser-assisted
// login flow. Only these profiles are eligible for remote key revoke
// on logout.
const SourceBrowser = "browser"

// SourceWithToken is set on profiles created via `lsh login --with-token`.
// On logout, only the local config entry is removed; the key is kept
// on the server because it may have been generated for use elsewhere.
const SourceWithToken = "with-token"
