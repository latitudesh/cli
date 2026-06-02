package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationLoginCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "login [DEPRECATED: api-token]",
		Short: "Authenticate with Latitude",
		Long: `Without arguments, opens your browser to authorize this CLI through the
Latitude dashboard and saves the resulting credential locally.

Pass --with-token <T> to skip the browser flow and use an existing token.

A positional <api-token> argument is still accepted for backward
compatibility but is deprecated and will be removed in a future release.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runOperationLogin,
	}
	cmd.Flags().String("with-token", "", "use an existing API token instead of the browser flow")
	cmd.Flags().String("profile", "", "save credentials under this profile name (default: team slug)")
	return cmd, nil
}

func runOperationLogin(cmd *cobra.Command, args []string) error {
	token, _ := cmd.Flags().GetString("with-token")
	profileName, _ := cmd.Flags().GetString("profile")

	if len(args) == 1 {
		if token != "" {
			return errors.New("cannot combine positional token with --with-token")
		}
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: passing the token as a positional argument is deprecated; use --with-token instead")
		token = args[0]
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	client := newAuthClient()

	if token != "" {
		return loginWithToken(ctx, client, token, profileName)
	}
	return loginViaBrowser(ctx, client, profileName)
}

// newAuthClient builds an authclient using the current hostname/scheme
// from viper. This is the same configuration the generated SDK uses,
// so dev overrides (--hostname / --scheme) flow through naturally.
func newAuthClient() *authclient.Client {
	hostname := viper.GetString("hostname")
	if hostname == "" {
		hostname = "api.latitude.sh"
	}
	scheme := viper.GetString("scheme")
	if scheme == "" {
		scheme = "https"
	}
	ua := fmt.Sprintf("lsh/%s", version.Version)
	return authclient.New(scheme+"://"+hostname, ua)
}

// saveProfile resolves the profile name (override > team slug > "default"),
// upserts it in the config file, and sets it as the default profile.
func saveProfile(override string, p config.Profile) (string, error) {
	f, err := config.Load()
	if err != nil {
		return "", err
	}
	name := override
	if name == "" {
		name = p.TeamSlug
	}
	if name == "" {
		name = "default"
	}
	f.SetProfile(name, p)
	f.DefaultProfile = name
	if err := config.Save(f); err != nil {
		return "", err
	}
	return name, nil
}
