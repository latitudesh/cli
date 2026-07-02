package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/spf13/cobra"
)

func makeOperationAuthLogoutCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove a stored profile and (for browser logins) revoke the API key",
		Long: `Removes the named profile from the local config. If the profile was
created by the browser-assisted login flow, also asks the API to
revoke the API key so it cannot be reused.

With --all, removes every stored profile (and revokes browser-created
keys on a best-effort basis).`,
		Example: `  # Logout the active profile (browser logins also revoke the key remotely)
  lsh auth logout

  # Logout a specific profile
  lsh auth logout --profile staging

  # Logout every stored profile
  lsh auth logout --all`,
		Args:         cobra.NoArgs,
		RunE:         runAuthLogout,
		SilenceUsage: true,
	}
	cmd.Flags().String("profile", "", "logout the named profile (default: active profile)")
	cmd.Flags().Bool("all", false, "logout every stored profile")
	return cmd, nil
}

func runAuthLogout(cmd *cobra.Command, _ []string) error {
	override, _ := cmd.Flags().GetString("profile")
	all, _ := cmd.Flags().GetBool("all")

	if all && override != "" {
		return errors.New("--profile and --all are mutually exclusive")
	}

	f, err := config.Load()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(f.Profiles) == 0 {
		fmt.Fprintln(out, "Nothing to do — no profiles are stored.")
		return nil
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	client := newAuthClient()

	if all {
		removed := f.SortedProfileNames()
		for _, name := range removed {
			revokeIfBrowserSourced(ctx, client, f.Profiles[name], name)
			f.RemoveProfile(name)
		}
		// Persist before confirming, so a failed write doesn't print
		// "Removed" for profiles that are still on disk.
		if err := config.Save(f); err != nil {
			return err
		}
		for _, name := range removed {
			fmt.Fprintf(out, "Removed profile %q\n", name)
		}
		return nil
	}

	name, profile, err := f.Resolve(override)
	if err != nil {
		if errors.Is(err, config.ErrProfileNotFound) {
			return errors.New("no matching profile to logout")
		}
		return err
	}
	wasDefault := f.DefaultProfile == name
	revokeIfBrowserSourced(ctx, client, profile, name)
	f.RemoveProfile(name)

	// If we just removed the active profile, fall back to another stored
	// one so the user keeps a usable context instead of landing on
	// "Not logged in" despite other profiles existing.
	var promoted string
	if wasDefault {
		promoted = f.EnsureDefault()
	}

	if err := config.Save(f); err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed profile %q\n", name)
	if promoted != "" {
		fmt.Fprintf(out, "Active profile is now %q.\n", promoted)
	}
	return nil
}

// revokeIfBrowserSourced asks the API to delete the key when the
// profile was created via browser login. Errors are surfaced as
// warnings but do not block local cleanup: the user can always retry
// the revoke from the dashboard.
func revokeIfBrowserSourced(ctx context.Context, client *authclient.Client, profile config.Profile, name string) {
	if profile.Source != config.SourceBrowser || profile.KeyID == "" || profile.Authorization == "" {
		return
	}
	if err := client.RevokeAPIKey(ctx, profile.Authorization, profile.KeyID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not revoke API key for profile %q: %v\n", name, err)
	}
}
