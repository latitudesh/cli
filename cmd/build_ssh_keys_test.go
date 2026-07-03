package cmd

import (
	"testing"
)

// TestSSHKeysAliasBackwardCompat verifies the kebab-case `ssh-keys` group keeps
// the legacy `ssh_keys` name as an alias so the old invocation still resolves.
func TestSSHKeysAliasBackwardCompat(t *testing.T) {
	if sshKeysCmd.Use != "ssh-keys" {
		t.Errorf("Use = %q, want ssh-keys", sshKeysCmd.Use)
	}

	found := false
	for _, a := range sshKeysCmd.Aliases {
		if a == "ssh_keys" {
			found = true
		}
	}
	if !found {
		t.Errorf("aliases = %v, want to include legacy ssh_keys", sshKeysCmd.Aliases)
	}

	// The alias must resolve back to the same command through cobra's lookup.
	if c, _, err := rootCmd.Find([]string{"ssh_keys"}); err != nil || c != sshKeysCmd {
		t.Errorf("rootCmd.Find([ssh_keys]) = (%v, %v), want the ssh-keys command", c, err)
	}
	if c, _, err := rootCmd.Find([]string{"ssh-keys"}); err != nil || c != sshKeysCmd {
		t.Errorf("rootCmd.Find([ssh-keys]) = (%v, %v), want the ssh-keys command", c, err)
	}
}

// TestSSHKeysSubcommandsRegistered guards the account-scoped CRUD wiring.
func TestSSHKeysSubcommandsRegistered(t *testing.T) {
	want := map[string]bool{"list": false, "get": false, "create": false, "update": false, "delete": false}
	for _, c := range sshKeysCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("ssh-keys missing subcommand %q", name)
		}
	}
}

// TestUserDataSubcommandsRegistered guards the account-scoped user data wiring.
func TestUserDataSubcommandsRegistered(t *testing.T) {
	if userDataCmd.Use != "user-data" {
		t.Errorf("Use = %q, want user-data", userDataCmd.Use)
	}
	want := map[string]bool{"list": false, "get": false, "create": false, "update": false, "delete": false}
	for _, c := range userDataCmd.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("user-data missing subcommand %q", name)
		}
	}
}

// TestProjectScopedGroupsWire verifies the project-scoped groups attach under
// the `projects` command and carry their subcommands.
func TestProjectScopedGroupsWire(t *testing.T) {
	sshKeys := newProjectSSHKeysCmd()
	wantSSH := map[string]bool{"list": false, "get": false, "create": false, "delete": false}
	for _, c := range sshKeys.Commands() {
		if _, ok := wantSSH[c.Name()]; ok {
			wantSSH[c.Name()] = true
		}
	}
	for name, seen := range wantSSH {
		if !seen {
			t.Errorf("projects ssh-keys missing subcommand %q", name)
		}
	}

	userData := newProjectUserDataCmd()
	wantUD := map[string]bool{"list": false, "get": false, "create": false, "update": false, "delete": false}
	for _, c := range userData.Commands() {
		if _, ok := wantUD[c.Name()]; ok {
			wantUD[c.Name()] = true
		}
	}
	for name, seen := range wantUD {
		if !seen {
			t.Errorf("projects user-data missing subcommand %q", name)
		}
	}
}
