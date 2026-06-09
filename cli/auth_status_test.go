package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runAuthStatusCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd, err := makeOperationAuthStatusCmd()
	if err != nil {
		t.Fatalf("make cmd: %v", err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.String()
}

func TestAuthStatusNotLoggedIn(t *testing.T) {
	withTempHome(t)
	t.Setenv("LATITUDESH_TOKEN", "")
	out := runAuthStatusCmd(t)
	if !strings.Contains(out, "Not logged in") {
		t.Fatalf("expected 'Not logged in', got\n%s", out)
	}
}

func TestAuthStatusListsAllProfiles(t *testing.T) {
	home := withTempHome(t)
	t.Setenv("LATITUDESH_TOKEN", "")
	writeConfig(t, home, `{"default_profile":"labs","profiles":{
		"labs":{"authorization":"x","team_name":"Labs","team_slug":"labs","email":"a@x.com","source":"browser"},
		"teamb":{"authorization":"y","team_name":"Team B","team_slug":"teamb","source":"with-token"}
	}}`)

	out := runAuthStatusCmd(t)
	for _, want := range []string{"Profile:    labs", "Profiles (* = default):", "labs", "teamb", "Switch with:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("auth status missing %q\n%s", want, out)
		}
	}
}

func TestAuthStatusHonorsEnvToken(t *testing.T) {
	withTempHome(t)
	t.Setenv("LATITUDESH_TOKEN", "ak_env")
	out := runAuthStatusCmd(t)
	if !strings.Contains(out, "LATITUDESH_TOKEN") {
		t.Fatalf("expected env-token source, got\n%s", out)
	}
}
