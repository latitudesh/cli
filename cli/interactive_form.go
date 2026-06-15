package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// canPromptInteractively reports whether a command may fall back to an
// interactive prompt for missing required input: stdin must be a TTY and
// the user must not have asked for fail-fast behavior via --no-input.
// Mirrors the gating used by resolveProjectFlag.
func canPromptInteractively(cmd *cobra.Command) bool {
	noInput, _ := cmd.Flags().GetBool("no-input")
	return !noInput && isInteractive()
}

// requiredFlagError is the fail-fast counterpart of the interactive
// prompts, shown when input is missing and prompting is not possible.
func requiredFlagError(flag string) error {
	return fmt.Errorf("--%s is required (pass --%s or run interactively without --no-input)", flag, flag)
}
