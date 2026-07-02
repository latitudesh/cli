package cmd

import (
	sshkeys "github.com/latitudesh/lsh/cmd/sshkeys"
	cobra "github.com/spf13/cobra"
)

func init() {
	sshKeysCmd.AddCommand(sshkeys.NewListCmd())
	sshKeysCmd.AddCommand(sshkeys.NewGetCmd())
	sshKeysCmd.AddCommand(sshkeys.NewCreateCmd())
	sshKeysCmd.AddCommand(sshkeys.NewUpdateCmd())
	sshKeysCmd.AddCommand(sshkeys.NewDeleteCmd())

	rootCmd.AddCommand(sshKeysCmd)
}

// sshKeysCmd is the account/team-scoped SSH key group. It replaces the legacy
// generated `ssh_keys` group (kept as a hidden alias for backward
// compatibility) and is built on the Go SDK.
var sshKeysCmd = &cobra.Command{
	Use:     "ssh-keys",
	Aliases: []string{"ssh_keys"},
	Short:   "Manage team SSH keys",
	Long: "Manage the team's SSH keys (account scope).\n\n" +
		"To manage the keys attached to a specific project, use\n" +
		"`lsh projects ssh-keys`.\n\n" +
		"The legacy `ssh_keys` name is still accepted as a hidden alias.",
	Example: `  lsh ssh-keys list
  lsh ssh-keys create --name laptop --public-key "ssh-ed25519 AAAA..."
  lsh ssh-keys get ssh_xxxxxxxx
  lsh ssh-keys update ssh_xxxxxxxx --name renamed
  lsh ssh-keys delete ssh_xxxxxxxx`,
}
