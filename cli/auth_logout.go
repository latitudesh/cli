package cli

import (
	"context"
	"errors"
	"fmt"

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
		Args: cobra.NoArgs,
		RunE: runAuthLogout,
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
	if len(f.Profiles) == 0 {
		fmt.Println("Nothing to do — no profiles are stored.")
		return nil
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	client := newAuthClient()

	if all {
		for name, profile := range f.Profiles {
			revokeIfBrowserSourced(ctx, client, profile, name)
			f.RemoveProfile(name)
			fmt.Printf("Removed profile %q\n", name)
		}
		return config.Save(f)
	}

	name, profile, err := f.Resolve(override)
	if err != nil {
		if errors.Is(err, config.ErrProfileNotFound) {
			return errors.New("no matching profile to logout")
		}
		return err
	}
	revokeIfBrowserSourced(ctx, client, profile, name)
	f.RemoveProfile(name)
	if err := config.Save(f); err != nil {
		return err
	}
	fmt.Printf("Removed profile %q\n", name)
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
		fmt.Printf("warning: could not revoke API key for profile %q: %v\n", name, err)
	}
}
