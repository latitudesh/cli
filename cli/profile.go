package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/latitudesh/lsh/internal/config"
	"github.com/latitudesh/lsh/internal/tui"
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
		Example: `  # Switch by profile name
  lsh profile use prod

  # Switch by team slug
  lsh profile use acme`,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runProfileUse,
		SilenceUsage: true,
	}
	return cmd, nil
}

func makeOperationProfileListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List the locally stored profiles (one per team)",
		Args:         cobra.NoArgs,
		RunE:         runProfileList,
		SilenceUsage: true,
	}
	return cmd, nil
}

func runProfileUse(cmd *cobra.Command, args []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	if len(f.Profiles) == 0 {
		return errors.New("no profiles stored — run `lsh login` first")
	}

	if len(args) == 0 {
		out := cmd.OutOrStdout()
		printProfileList(out, f)
		fmt.Fprintln(out)
		return errors.New("provide a profile name or team slug to switch to (see list above)")
	}

	target := args[0]

	// Primary: exact profile name match.
	if profile, ok := f.Profiles[target]; ok {
		f.DefaultProfile = target
		if err := config.Save(f); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Active profile is now %q (team: %s)\n", target, formatTeam(profile))
		return nil
	}

	// Fallback: match by team identifier so users can pick by team slug/id.
	for name, profile := range f.Profiles {
		if profile.TeamID == target || profile.TeamSlug == target {
			f.DefaultProfile = name
			if err := config.Save(f); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Active profile is now %q (team: %s)\n", name, formatTeam(profile))
			return nil
		}
	}
	return fmt.Errorf("no local profile matches %q — run `lsh login` to add it", target)
}

func runProfileList(cmd *cobra.Command, _ []string) error {
	f, err := config.Load()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(f.Profiles) == 0 {
		fmt.Fprintln(out, "No profiles stored. Run `lsh login` to authenticate.")
		return nil
	}
	printProfileList(out, f)
	return nil
}

func printProfileList(w io.Writer, f *config.File) {
	names := f.SortedProfileNames()

	// Size the columns to their content so long names/teams stay aligned.
	nameW, teamW := len("PROFILE"), len("TEAM")
	for _, name := range names {
		if len(name) > nameW {
			nameW = len(name)
		}
		if t := teamLabel(f.Profiles[name]); len(t) > teamW {
			teamW = len(t)
		}
	}

	fmt.Fprintf(w, "  %-*s  %-*s  %s\n", nameW, "PROFILE", teamW, "TEAM", "EMAIL")
	for _, name := range names {
		p := f.Profiles[name]
		active := name == f.DefaultProfile
		marker := "  "
		if active {
			marker = "* "
		}
		line := fmt.Sprintf("%s%-*s  %-*s  %s", marker, nameW, name, teamW, teamLabel(p), emptyAsDash(p.Email))
		if active {
			line = tui.FocusedStyle.Render(line)
		}
		fmt.Fprintln(w, line)
	}
	if f.DefaultProfile != "" {
		fmt.Fprintln(w, "\n* = active profile")
	}
}
