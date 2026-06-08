package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/spf13/cobra"
)

func makeOperationAuthStatusCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current authentication context",
		Long: `Prints the email, team, default project, key name and source of the
profile that lsh would use right now. Honors --profile and the
LSH_PROFILE / LATITUDESH_TOKEN environment variables.`,
		Args: cobra.NoArgs,
		RunE: runAuthStatus,
	}
	cmd.Flags().String("profile", "", "show status for this profile (overrides default)")
	return cmd, nil
}

func runAuthStatus(cmd *cobra.Command, _ []string) error {
	override, _ := cmd.Flags().GetString("profile")

	// Mirror HydrateFromActiveProfile: LATITUDESH_TOKEN takes precedence
	// over stored profiles, so report it here too instead of falling
	// through to "Not logged in".
	if os.Getenv("LATITUDESH_TOKEN") != "" {
		fmt.Println("Profile:    - (using LATITUDESH_TOKEN)")
		fmt.Println("Email:      -")
		fmt.Println("Team:       -")
		fmt.Println("API key:    -")
		fmt.Println("Source:     environment (LATITUDESH_TOKEN)")
		return nil
	}

	f, err := config.Load()
	if err != nil {
		return err
	}

	name, profile, err := f.Resolve(override)
	if err != nil {
		if errors.Is(err, config.ErrProfileNotFound) {
			fmt.Println("Not logged in. Run `lsh login` to authenticate.")
			return nil
		}
		return err
	}

	fmt.Printf("Profile:    %s%s\n", name, defaultMarker(f.DefaultProfile, name))
	fmt.Printf("Email:      %s\n", emptyAsDash(profile.Email))
	fmt.Printf("Team:       %s\n", formatTeam(profile))
	fmt.Printf("API key:    %s\n", emptyAsDash(profile.KeyName))
	fmt.Printf("Source:     %s\n", emptyAsDash(profile.Source))
	return nil
}

func defaultMarker(defaultName, current string) string {
	if defaultName == current {
		return " (default)"
	}
	return ""
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
