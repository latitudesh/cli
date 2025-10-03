package cli

import (
	"fmt"
	"os"
	"path"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func makeOperationLoginCmd() (*cobra.Command, error) {
	// loginCmd represents the login command
	cmd := &cobra.Command{
		Use:   "login [api-token]",
		Short: "Set your Auth Token",
		Long: `login will create a configuration file and save your API authentication 
token in it, allowing it to be used when interacting with the API.

The configuration will be stored in your home directory and also copied to
root's directory so sudo commands work seamlessly.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runOperationLogin,
	}

	return cmd, nil
}

func runOperationLogin(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		cmd.Help()
		os.Exit(0)
	}

	home, err := homedir.Dir()
	cobra.CheckErr(err)

	folderPath := path.Join(home, ".config", exeName)

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		os.MkdirAll(folderPath, 0700)
	}

	configPath := path.Join(folderPath, "config.json")
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	viper.Set("API-Version", "2023-06-01")
	viper.Set("authorization", args[0])
	viper.WriteConfig()

	fmt.Println("✅ Success! Configuration file updated.")
	fmt.Printf("   Config stored at: %s\n", configPath)

	// Also copy to root's config directory so sudo commands work
	rootFolderPath := path.Join("/root", ".config", exeName)
	rootConfigPath := path.Join(rootFolderPath, "config.json")

	// Try to create root's config directory and copy the file
	if err := os.MkdirAll(rootFolderPath, 0700); err == nil {
		if input, err := os.ReadFile(configPath); err == nil {
			if err := os.WriteFile(rootConfigPath, input, 0600); err == nil {
				fmt.Printf("   Also copied to: %s (for sudo commands)\n", rootConfigPath)
			}
		}
	}

	return nil
}
