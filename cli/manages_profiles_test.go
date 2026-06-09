package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestManagesProfiles(t *testing.T) {
	root := &cobra.Command{Use: "lsh"}

	login := &cobra.Command{Use: "login"}
	root.AddCommand(login)

	auth := &cobra.Command{Use: "auth"}
	authStatus := &cobra.Command{Use: "status"}
	auth.AddCommand(authStatus)
	root.AddCommand(auth)

	profile := &cobra.Command{Use: "profile"}
	profileUse := &cobra.Command{Use: "use"}
	profile.AddCommand(profileUse)
	root.AddCommand(profile)

	servers := &cobra.Command{Use: "servers"}
	serversList := &cobra.Command{Use: "list"}
	servers.AddCommand(serversList)
	root.AddCommand(servers)

	cases := []struct {
		cmd  *cobra.Command
		want bool
	}{
		{login, true},
		{authStatus, true}, // nested under auth
		{auth, true},
		{profileUse, true}, // nested under profile
		{servers, false},
		{serversList, false}, // generated API command → must hydrate
	}
	for _, c := range cases {
		if got := managesProfiles(c.cmd); got != c.want {
			t.Fatalf("managesProfiles(%q) = %v, want %v", c.cmd.Name(), got, c.want)
		}
	}
}
