package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/spf13/cobra"
)

func makeOperationProfileCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage local CLI profiles (one per team you are logged into)",
	}
	useCmd, err := makeOperationProfileUseCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(useCmd)

	listCmd, err := makeOperationProfileListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	return cmd, nil
}

func makeOperationProfileUseCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "use [profile-name-or-team-slug]",
		Short: "Set the active profile (and therefore the active team)",
		Long: "Picks a locally stored profile and marks it as the default. The " +
			"argument can be the profile name or the slug/id of the team it is " +
			"bound to. Run `lsh profile list` to see what is stored locally.",
		Args: cobra.MaximumNArgs(1),
		RunE: runProfileUse,
	}
	return cmd, nil
}

func makeOperationProfileListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the locally stored profiles (one per team)",
		Args:  cobra.NoArgs,
		RunE:  runProfileList,
	}
	return cmd, nil
}

func runProfileUse(_ *cobra.Command, args []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	if len(f.Profiles) == 0 {
		return errors.New("no profiles stored — run `lsh login` first")
	}

	if len(args) == 0 {
		printProfileList(f)
		fmt.Println()
		return errors.New("provide a profile name or team slug to switch to (see list above)")
	}

	target := args[0]

	// Primary: exact profile name match.
	if profile, ok := f.Profiles[target]; ok {
		f.DefaultProfile = target
		if err := config.Save(f); err != nil {
			return err
		}
		fmt.Printf("Active profile is now %q (team: %s)\n", target, formatTeam(profile))
		return nil
	}

	// Fallback: match by team identifier so users can pick by team slug/id.
	for name, profile := range f.Profiles {
		if profile.TeamID == target || profile.TeamSlug == target {
			f.DefaultProfile = name
			if err := config.Save(f); err != nil {
				return err
			}
			fmt.Printf("Active profile is now %q (team: %s)\n", name, formatTeam(profile))
			return nil
		}
	}
	return fmt.Errorf("no local profile matches %q — run `lsh login` to add it", target)
}

func runProfileList(_ *cobra.Command, _ []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	if len(f.Profiles) == 0 {
		fmt.Println("No profiles stored. Run `lsh login` to authenticate.")
		return nil
	}
	printProfileList(f)
	return nil
}

func printProfileList(f *config.File) {
	names := make([]string, 0, len(f.Profiles))
	for name := range f.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Printf("%-20s %-30s %s\n", "PROFILE", "TEAM", "EMAIL")
	for _, name := range names {
		p := f.Profiles[name]
		marker := ""
		if name == f.DefaultProfile {
			marker = " *"
		}
		fmt.Printf("%-20s %-30s %s\n", name+marker, formatTeam(p), emptyAsDash(p.Email))
	}
	if f.DefaultProfile != "" {
		fmt.Println("\n* = active profile")
	}
}
