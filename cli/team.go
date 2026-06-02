package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/spf13/cobra"
)

func makeOperationTeamCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Switch between the teams you are logged into",
	}
	useCmd, err := makeOperationTeamUseCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(useCmd)

	listCmd, err := makeOperationTeamListCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(listCmd)

	return cmd, nil
}

func makeOperationTeamUseCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "use [team-id-or-slug]",
		Short: "Set the active team (changes the default profile)",
		Long: "Picks the locally stored profile whose team matches the given id or " +
			"slug and marks it as the default profile. Run `lsh team list` to see " +
			"which teams you are logged into.",
		Args: cobra.MaximumNArgs(1),
		RunE: runTeamUse,
	}
	return cmd, nil
}

func makeOperationTeamListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the teams you are logged into (one profile per team)",
		Args:  cobra.NoArgs,
		RunE:  runTeamList,
	}
	return cmd, nil
}

func runTeamUse(_ *cobra.Command, args []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	if len(f.Profiles) == 0 {
		return errors.New("no profiles stored — run `lsh login` first")
	}

	if len(args) == 0 {
		printTeamList(f)
		fmt.Println()
		return errors.New("provide the team id or slug to switch to (see list above)")
	}

	target := args[0]
	for name, profile := range f.Profiles {
		if profile.TeamID == target || profile.TeamSlug == target {
			f.DefaultProfile = name
			if err := config.Save(f); err != nil {
				return err
			}
			fmt.Printf("Active team is now %s (profile: %s)\n", formatTeam(profile), name)
			return nil
		}
	}
	return fmt.Errorf("no local profile matches team %q — run `lsh login` to add it", target)
}

func runTeamList(_ *cobra.Command, _ []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	if len(f.Profiles) == 0 {
		fmt.Println("No profiles stored. Run `lsh login` to authenticate.")
		return nil
	}
	printTeamList(f)
	return nil
}

func printTeamList(f *config.File) {
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
