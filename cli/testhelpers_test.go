package cli

import (
	"os"
	"path/filepath"
	"testing"

	homedir "github.com/mitchellh/go-homedir"
)

// withTempHome points the config path at a throwaway directory for the
// duration of the test. config.Path resolves via homedir.Dir (which reads
// $HOME and caches), so we set $HOME and clear the cache before and after.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	homedir.Reset()
	t.Cleanup(homedir.Reset)
	return dir
}

// writeConfig seeds the isolated config file with raw JSON.
func writeConfig(t *testing.T, home, body string) {
	t.Helper()
	p := filepath.Join(home, ".config", "lsh", "config.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readConfig returns the raw isolated config file contents.
func readConfig(t *testing.T, home string) string {
	t.Helper()
	p := filepath.Join(home, ".config", "lsh", "config.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(b)
}
