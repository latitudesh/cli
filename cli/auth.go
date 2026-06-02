package cli

import "github.com/spf13/cobra"

func makeOperationAuthCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect and manage your authentication state",
	}
	statusCmd, err := makeOperationAuthStatusCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(statusCmd)

	logoutCmd, err := makeOperationAuthLogoutCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(logoutCmd)

	return cmd, nil
}
