package cli

import (
	"testing"

	"github.com/latitudesh/lsh/internal/config"
)

func TestSaveProfileUsesTeamSlugAndSetsDefault(t *testing.T) {
	withTempHome(t)

	name, err := saveProfile("", config.Profile{Authorization: "t", TeamSlug: "acme", TeamName: "Acme"})
	if err != nil {
		t.Fatalf("saveProfile: %v", err)
	}
	if name != "acme" {
		t.Fatalf("expected name from team slug, got %q", name)
	}
	f, _ := config.Load()
	if f.DefaultProfile != "acme" {
		t.Fatalf("first profile should become default, got %q", f.DefaultProfile)
	}
}

func TestSaveProfileOverrideWins(t *testing.T) {
	withTempHome(t)

	name, err := saveProfile("staging", config.Profile{Authorization: "t", TeamSlug: "acme"})
	if err != nil {
		t.Fatalf("saveProfile: %v", err)
	}
	if name != "staging" {
		t.Fatalf("expected override name, got %q", name)
	}
}

func TestSaveProfileFallbackToDefaultWhenNoDefaultExists(t *testing.T) {
	withTempHome(t)

	// No override, no team slug, and no existing "default" → allowed.
	name, err := saveProfile("", config.Profile{Authorization: "t"})
	if err != nil {
		t.Fatalf("saveProfile: %v", err)
	}
	if name != "default" {
		t.Fatalf("expected fallback name 'default', got %q", name)
	}
}

func TestSaveProfileRefusesToClobberExistingDefault(t *testing.T) {
	home := withTempHome(t)
	writeConfig(t, home, `{"default_profile":"default","profiles":{"default":{"authorization":"original"}}}`)

	// No override and no team slug would fall back to "default" and silently
	// overwrite the existing credential — must error instead.
	_, err := saveProfile("", config.Profile{Authorization: "new"})
	if err == nil {
		t.Fatal("expected error to avoid clobbering existing default profile")
	}
	f, _ := config.Load()
	if f.Profiles["default"].Authorization != "original" {
		t.Fatalf("existing default credential must be untouched, got %q", f.Profiles["default"].Authorization)
	}
}
