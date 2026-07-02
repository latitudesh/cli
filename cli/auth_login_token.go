package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/config"
)

// loginWithToken validates an existing API token by calling
// GET /user/profile and persists it under a profile.
func loginWithToken(ctx context.Context, client *authclient.Client, token, profileOverride string) error {
	if token == "" {
		return errors.New("--with-token requires a non-empty value")
	}

	profileResp, err := client.GetUserProfile(ctx, token)
	if err != nil {
		var httpErr *authclient.HTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == 401 || httpErr.StatusCode == 403) {
			return errors.New("token rejected by the API — check the value and try again")
		}
		return fmt.Errorf("could not validate token: %w", err)
	}

	// The user profile payload doesn't include the team. Fetch it
	// separately — GET /team is scoped to the token's membership
	// server-side, so it returns exactly the team this token is bound to.
	team, err := client.GetCurrentTeam(ctx, token)
	if err != nil {
		return fmt.Errorf("could not fetch team for this token: %w", err)
	}

	profile := config.Profile{
		Authorization: token,
		Email:         profileResp.Email,
		Source:        config.SourceWithToken,
		APIVersion:    defaultAPIVersion(),
	}
	if team != nil {
		profile.TeamID = team.ID
		profile.TeamName = team.Name
		profile.TeamSlug = team.Slug
	}

	name, err := saveProfile(profileOverride, profile)
	if err != nil {
		return fmt.Errorf("could not save profile: %w", err)
	}

	fmt.Printf("✅ Logged in as %s on team %s (profile: %s)\n", profile.Email, profile.TeamName, name)
	return nil
}
