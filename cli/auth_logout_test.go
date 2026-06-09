package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/latitudesh/lsh/internal/config"
)

func runAuthLogoutCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd, err := makeOperationAuthLogoutCmd()
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

func TestLogoutPromotesFallbackDefault(t *testing.T) {
	home := withTempHome(t)
	// Two with-token profiles (no remote revoke), teamA active.
	writeConfig(t, home, `{"default_profile":"teamA","profiles":{
		"teamA":{"authorization":"a","team_slug":"teamA","source":"with-token"},
		"teamB":{"authorization":"b","team_slug":"teamB","source":"with-token"}
	}}`)

	out := runAuthLogoutCmd(t)
	if !strings.Contains(out, `Active profile is now "teamB"`) {
		t.Fatalf("expected fallback promotion to teamB, got\n%s", out)
	}
	f, _ := config.Load()
	if _, gone := f.Profiles["teamA"]; gone {
		t.Fatal("teamA should be removed")
	}
	if f.DefaultProfile != "teamB" {
		t.Fatalf("expected default promoted to teamB, got %q", f.DefaultProfile)
	}
}

func TestLogoutAllRemovesEverything(t *testing.T) {
	home := withTempHome(t)
	writeConfig(t, home, `{"default_profile":"teamA","profiles":{
		"teamA":{"authorization":"a","team_slug":"teamA","source":"with-token"},
		"teamB":{"authorization":"b","team_slug":"teamB","source":"with-token"}
	}}`)

	out := runAuthLogoutCmd(t, "--all")
	for _, want := range []string{`Removed profile "teamA"`, `Removed profile "teamB"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("logout --all missing %q\n%s", want, out)
		}
	}
	f, _ := config.Load()
	if len(f.Profiles) != 0 {
		t.Fatalf("expected all profiles removed, got %+v", f.Profiles)
	}
}

func TestLogoutLastProfileNoPromotion(t *testing.T) {
	home := withTempHome(t)
	writeConfig(t, home, `{"default_profile":"only","profiles":{"only":{"authorization":"x","team_slug":"only","source":"with-token"}}}`)

	out := runAuthLogoutCmd(t)
	if strings.Contains(out, "Active profile is now") {
		t.Fatalf("no promotion expected when removing the last profile, got\n%s", out)
	}
	f, _ := config.Load()
	if len(f.Profiles) != 0 || f.DefaultProfile != "" {
		t.Fatalf("expected empty config, got %+v", f)
	}
}
