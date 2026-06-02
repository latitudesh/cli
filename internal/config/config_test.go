package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withTempHome substitutes the package-level homeDir resolver so
// Load/Save touch only the test directory.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := homeDir
	homeDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { homeDir = original })
	return dir
}

func writeFile(t *testing.T, p string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEmptyWhenMissing(t *testing.T) {
	withTempHome(t)
	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(f.Profiles) != 0 || f.DefaultProfile != "" {
		t.Fatalf("expected empty config, got %+v", f)
	}
}

func TestLoadMigratesLegacyTopLevelToken(t *testing.T) {
	home := withTempHome(t)
	cfgPath := filepath.Join(home, ".config", "lsh", "config.json")
	writeFile(t, cfgPath, `{"Authorization":"old-token","API-Version":"2023-06-01","hostname":"api.latitude.sh"}`)

	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f.DefaultProfile != "default" {
		t.Fatalf("expected default_profile=default, got %q", f.DefaultProfile)
	}
	p, ok := f.Profiles["default"]
	if !ok {
		t.Fatal("expected migrated profile 'default'")
	}
	if p.Authorization != "old-token" {
		t.Fatalf("expected migrated token, got %q", p.Authorization)
	}
	if p.Source != SourceWithToken {
		t.Fatalf("expected source=%s, got %q", SourceWithToken, p.Source)
	}
	if p.APIVersion != "2023-06-01" {
		t.Fatalf("expected api_version migrated, got %q", p.APIVersion)
	}
}

func TestLoadDoesNotOverwriteExistingProfiles(t *testing.T) {
	home := withTempHome(t)
	cfgPath := filepath.Join(home, ".config", "lsh", "config.json")
	// Both legacy AND new fields present — should keep the new profiles.
	body := `{
        "Authorization":"legacy-token",
        "default_profile":"acme",
        "profiles":{"acme":{"authorization":"ak_new","team_slug":"acme","source":"browser"}}
    }`
	writeFile(t, cfgPath, body)

	f, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f.DefaultProfile != "acme" {
		t.Fatalf("expected default_profile=acme, got %q", f.DefaultProfile)
	}
	if _, ok := f.Profiles["default"]; ok {
		t.Fatal("legacy migration should not run when profiles already exist")
	}
	if f.Profiles["acme"].Authorization != "ak_new" {
		t.Fatalf("expected ak_new, got %q", f.Profiles["acme"].Authorization)
	}
}

func TestSaveCreatesDirAndRoundtrips(t *testing.T) {
	home := withTempHome(t)
	f := &File{
		DefaultProfile: "acme",
		Profiles: map[string]Profile{
			"acme": {Authorization: "ak_xxx", TeamSlug: "acme", Email: "u@x.com", Source: SourceBrowser},
		},
	}
	if err := Save(f); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfgPath := filepath.Join(home, ".config", "lsh", "config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var dec File
	if err := json.Unmarshal(raw, &dec); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dec.DefaultProfile != "acme" || dec.Profiles["acme"].Authorization != "ak_xxx" {
		t.Fatalf("unexpected on-disk content: %s", raw)
	}
}

func TestResolveOrder(t *testing.T) {
	f := &File{
		DefaultProfile: "default-team",
		Profiles: map[string]Profile{
			"default-team": {Authorization: "default-tok", TeamSlug: "default-team"},
			"other-team":   {Authorization: "other-tok", TeamSlug: "other-team"},
		},
	}

	t.Run("explicit override wins", func(t *testing.T) {
		t.Setenv("LSH_PROFILE", "default-team")
		name, p, err := f.Resolve("other-team")
		if err != nil {
			t.Fatal(err)
		}
		if name != "other-team" || p.Authorization != "other-tok" {
			t.Fatalf("unexpected: %s/%s", name, p.Authorization)
		}
	})

	t.Run("LSH_PROFILE beats default", func(t *testing.T) {
		t.Setenv("LSH_PROFILE", "other-team")
		name, _, err := f.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		if name != "other-team" {
			t.Fatalf("expected other-team, got %s", name)
		}
	})

	t.Run("falls back to default_profile", func(t *testing.T) {
		t.Setenv("LSH_PROFILE", "")
		name, _, err := f.Resolve("")
		if err != nil {
			t.Fatal(err)
		}
		if name != "default-team" {
			t.Fatalf("expected default-team, got %s", name)
		}
	})

	t.Run("ErrProfileNotFound when nothing resolves", func(t *testing.T) {
		empty := &File{Profiles: map[string]Profile{}}
		_, _, err := empty.Resolve("")
		if err == nil {
			t.Fatal("expected error")
		}
		if err != ErrProfileNotFound {
			t.Fatalf("expected ErrProfileNotFound, got %v", err)
		}
	})
}

func TestSetAndRemoveProfile(t *testing.T) {
	f := &File{}
	f.SetProfile("a", Profile{Authorization: "ta"})
	if f.DefaultProfile != "a" {
		t.Fatalf("expected default to be set on first insert, got %q", f.DefaultProfile)
	}
	f.SetProfile("b", Profile{Authorization: "tb"})
	if f.DefaultProfile != "a" {
		t.Fatalf("expected default to stay on first insert, got %q", f.DefaultProfile)
	}
	f.RemoveProfile("a")
	if _, ok := f.Profiles["a"]; ok {
		t.Fatal("profile a should be gone")
	}
	if f.DefaultProfile != "" {
		t.Fatalf("expected default cleared after removing default, got %q", f.DefaultProfile)
	}
}
