// Package config loads and persists the lsh config file with support
// for multiple profiles (one per team), env var overrides, and a
// migration path from the previous single-token format.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	homedir "github.com/mitchellh/go-homedir"
)

// File holds the on-disk shape of ~/.config/lsh/config.json.
type File struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`

	// Hostname/Scheme/BasePath are kept at the top level (not per
	// profile) — they apply globally and are usually only overridden
	// for development. Existing config files use lowercase keys.
	Hostname string `json:"hostname,omitempty"`
	Scheme   string `json:"scheme,omitempty"`
	BasePath string `json:"base_path,omitempty"`

	// Output / Json control rendering for the existing generated
	// commands. Kept top-level for backward compatibility.
	Output string `json:"output,omitempty"`
	JSON   bool   `json:"json,omitempty"`
}

// ErrProfileNotFound is returned when a profile name is requested but
// is not defined in the config file.
var ErrProfileNotFound = errors.New("config: profile not found")

const (
	dirPerm  = 0o700
	filePerm = 0o600
	dirName  = ".config"
	exeName  = "lsh"
)

// homeDir is the package-level home directory resolver. Tests
// substitute this with a function that returns a temporary directory.
var homeDir = homedir.Dir

// Path returns the absolute path to the config file (existing or not).
func Path() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName, exeName, "config.json"), nil
}

// Load reads the config file from disk. If it does not exist, returns
// a zero-valued *File (callers can then populate and Save).
func Load() (*File, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Profiles: map[string]Profile{}}, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", p, err)
	}
	f := &File{}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", p, err)
	}
	if migrateLegacyInto(f, data) {
		// Persist the migrated format so the on-disk file stops being
		// legacy. Best-effort: a read-only environment shouldn't turn a
		// successful load into an error — the in-memory result is correct
		// regardless.
		_ = Save(f)
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	return f, nil
}

// Save writes the config back to disk, creating ~/.config/lsh if needed.
func Save(f *File) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	// Write to a temp file in the same directory, then atomically rename
	// over the target. A crash mid-write can't truncate the existing
	// credential store this way (os.WriteFile would).
	tmp, err := os.CreateTemp(filepath.Dir(p), ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("config: write temp: %w", err)
	}
	if err := tmp.Chmod(filePerm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("config: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("config: close temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}

// SetProfile inserts or replaces a profile by name.
func (f *File) SetProfile(name string, p Profile) {
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	f.Profiles[name] = p
	if f.DefaultProfile == "" {
		f.DefaultProfile = name
	}
}

// RemoveProfile deletes a profile by name. If the deleted profile was
// the default, DefaultProfile is cleared (callers may pick a new one).
func (f *File) RemoveProfile(name string) {
	delete(f.Profiles, name)
	if f.DefaultProfile == name {
		f.DefaultProfile = ""
	}
}

// SortedProfileNames returns the stored profile names in alphabetical order.
func (f *File) SortedProfileNames() []string {
	names := make([]string, 0, len(f.Profiles))
	for name := range f.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EnsureDefault promotes the alphabetically-first profile to default when
// no default is set but profiles still exist (e.g. after the active
// profile is logged out). Returns the chosen name, or "" if none remain.
func (f *File) EnsureDefault() string {
	if f.DefaultProfile != "" {
		return f.DefaultProfile
	}
	names := f.SortedProfileNames()
	if len(names) == 0 {
		return ""
	}
	f.DefaultProfile = names[0]
	return f.DefaultProfile
}

// Resolve returns the active profile name and its data based on the
// supplied override (e.g. `--profile <name>` flag value), the
// LSH_PROFILE env var, and finally DefaultProfile.
func (f *File) Resolve(override string) (string, Profile, error) {
	name := override
	if name == "" {
		name = os.Getenv("LSH_PROFILE")
	}
	if name == "" {
		name = f.DefaultProfile
	}
	if name == "" {
		return "", Profile{}, ErrProfileNotFound
	}
	p, ok := f.Profiles[name]
	if !ok {
		return name, Profile{}, ErrProfileNotFound
	}
	return name, p, nil
}
