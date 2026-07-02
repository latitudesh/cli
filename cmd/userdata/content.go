package userdata

import (
	"encoding/base64"

	cobra "github.com/spf13/cobra"
)

// ResolveContent returns the base64-encoded content to send to the API.
//
// The API stores user data content base64-encoded. To keep the CLI friendly,
// --content takes plain text and is encoded here; --content-base64 takes a
// value that is already encoded and is passed through unchanged (useful for
// piping file contents that are not valid UTF-8 text on the shell).
//
// It is shared with the project-scoped commands (cmd/projectuserdata), which
// operate on the same resource.
func ResolveContent(cmd *cobra.Command) (string, bool) {
	if cmd.Flags().Changed("content-base64") {
		v, _ := cmd.Flags().GetString("content-base64")
		return v, true
	}
	if cmd.Flags().Changed("content") {
		v, _ := cmd.Flags().GetString("content")
		return base64.StdEncoding.EncodeToString([]byte(v)), true
	}
	return "", false
}

// RegisterContentFlags adds the mutually-exclusive content flags.
func RegisterContentFlags(cmd *cobra.Command) {
	cmd.Flags().String("content", "", "Plain-text content (base64-encoded before sending)")
	cmd.Flags().String("content-base64", "", "Already base64-encoded content (sent as-is)")
	cmd.MarkFlagsMutuallyExclusive("content", "content-base64")
}
