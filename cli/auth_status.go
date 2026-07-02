package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/latitudesh/lsh/internal/authclient"
	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/tui"
	"github.com/spf13/cobra"
)

func makeOperationAuthStatusCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current authentication context",
		Long: `Prints the email, team, default project, key name and source of the
profile that lsh would use right now, followed by every other stored
profile. Honors --profile and the LSH_PROFILE / LATITUDESH_TOKEN
environment variables.

With --check, each stored profile's token is validated against the API
and shown as valid / invalid.`,
		Example: `  # Show the active profile
  lsh auth status

  # Show a specific profile without changing the default
  lsh auth status --profile staging`,
		Args:         cobra.NoArgs,
		RunE:         runAuthStatus,
		SilenceUsage: true,
	}
	cmd.Flags().String("profile", "", "show status for this profile (overrides default)")
	cmd.Flags().Bool("check", false, "validate each stored profile's token against the API")
	return cmd, nil
}

func runAuthStatus(cmd *cobra.Command, _ []string) error {
	override, _ := cmd.Flags().GetString("profile")
	check, _ := cmd.Flags().GetBool("check")

	f, err := config.Load()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var client *authclient.Client
	if check {
		client = newAuthClient()
	}

	out := cmd.OutOrStdout()

	// Active context. LATITUDESH_TOKEN takes precedence over stored
	// profiles (mirrors HydrateFromActiveProfile), so report it first.
	switch {
	case os.Getenv("LATITUDESH_TOKEN") != "":
		fmt.Fprintln(out, "Profile:    - (using LATITUDESH_TOKEN)")
		fmt.Fprintln(out, "Email:      -")
		fmt.Fprintln(out, "Team:       -")
		fmt.Fprintln(out, "API key:    -")
		fmt.Fprintln(out, "Source:     environment (LATITUDESH_TOKEN)")
		if check {
			envProfile := config.Profile{Authorization: os.Getenv("LATITUDESH_TOKEN")}
			fmt.Fprintf(out, "Token:      %s\n", styleValidity(profileValidity(ctx, client, envProfile)))
		}
	default:
		name, profile, rErr := f.Resolve(override)
		if rErr != nil && !errors.Is(rErr, config.ErrProfileNotFound) {
			return rErr
		}
		if errors.Is(rErr, config.ErrProfileNotFound) {
			if len(f.Profiles) == 0 {
				fmt.Fprintln(out, "Not logged in. Run `lsh login` to authenticate.")
				return nil
			}
			fmt.Fprintln(out, "No active profile selected. Run `lsh profile use <profile name>`.")
		} else {
			fmt.Fprintf(out, "Profile:    %s%s\n", name, defaultMarker(f.DefaultProfile, name))
			fmt.Fprintf(out, "Email:      %s\n", emptyAsDash(profile.Email))
			fmt.Fprintf(out, "Team:       %s\n", formatTeam(profile))
			fmt.Fprintf(out, "API key:    %s\n", emptyAsDash(profile.KeyName))
			fmt.Fprintf(out, "Source:     %s\n", emptyAsDash(profile.Source))
			if check {
				fmt.Fprintf(out, "Token:      %s\n", styleValidity(profileValidity(ctx, client, profile)))
			}
		}
	}

	printStoredProfiles(out, ctx, f, client)
	return nil
}

// printStoredProfiles lists every stored profile, marking the default with
// "*". When client is non-nil, each profile's token is validated.
func printStoredProfiles(w io.Writer, ctx context.Context, f *config.File, client *authclient.Client) {
	names := f.SortedProfileNames()
	if len(names) == 0 {
		return
	}
	fmt.Fprintln(w, "\nProfiles (* = default):")
	width := 0
	for _, n := range names {
		if l := len(n) + 2; l > width { // +2 for the surrounding parentheses
			width = l
		}
	}
	for _, n := range names {
		p := f.Profiles[n]
		active := n == f.DefaultProfile
		marker := "  "
		if active {
			marker = "* "
		}
		line := fmt.Sprintf("%s%-*s %s", marker, width, "("+n+")", teamLabel(p))
		if active {
			line = tui.FocusedStyle.Render(line)
		}
		// Validity keeps its own color, appended after styling so its ANSI
		// codes aren't nested inside the active-row highlight.
		if client != nil {
			line += "   [" + styleValidity(profileValidity(ctx, client, p)) + "]"
		}
		fmt.Fprintln(w, line)
	}

	if len(names) > 1 {
		fmt.Fprintln(w, tui.HelpStyle.Render("Switch with: lsh profile use <profile name>"))
	}
}

// styleValidity colors a token-validity word: green=valid, red=invalid,
// amber for everything else (no token / check failed).
func styleValidity(status string) string {
	switch status {
	case "valid":
		return tui.SuccessStyle.Render(status)
	case "invalid":
		return tui.ErrorStyle.Render(status)
	default:
		return tui.WarningStyle.Render(status)
	}
}

// profileValidity validates a profile's token via GET /user/profile.
// Returns "valid", "invalid" (401/403), "no token", or "unknown" when the
// check itself failed (e.g. network error).
func profileValidity(ctx context.Context, client *authclient.Client, p config.Profile) string {
	if p.Authorization == "" {
		return "no token"
	}
	if _, err := client.GetUserProfile(ctx, p.Authorization); err != nil {
		var httpErr *authclient.HTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == 401 || httpErr.StatusCode == 403) {
			return "invalid"
		}
		return "unknown"
	}
	return "valid"
}

func defaultMarker(defaultName, current string) string {
	if defaultName == current {
		return " (default)"
	}
	return ""
}

// teamLabel returns the human team name (falling back to the slug, then
// "-"). Unlike formatTeam it omits the slug, since the profile list already
// shows the profile name — usually the slug — in parentheses.
func teamLabel(p config.Profile) string {
	if p.TeamName != "" {
		return p.TeamName
	}
	if p.TeamSlug != "" {
		return p.TeamSlug
	}
	return "-"
}

func formatTeam(p config.Profile) string {
	if p.TeamName == "" && p.TeamSlug == "" {
		return "-"
	}
	if p.TeamSlug == "" {
		return p.TeamName
	}
	if p.TeamName == "" {
		return p.TeamSlug
	}
	return fmt.Sprintf("%s (%s)", p.TeamName, p.TeamSlug)
}

func emptyAsDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
