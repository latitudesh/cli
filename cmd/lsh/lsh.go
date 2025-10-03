// Package to configure options that should be acessible througout all commands
package lsh

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path"

	sdk "github.com/latitudesh/latitudesh-go"
	"github.com/latitudesh/lsh/internal/version"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Dry run flag
var DryRun bool

// Debug flag indicating that cli should output debug logs
var Debug bool

var UserAgent = fmt.Sprintf("Latitude-CLI: %s", version.Version)

var ExeName = "lsh"

// LogDebugf writes debug log to stdout
func LogDebugf(format string, v ...interface{}) {
	if !Debug {
		return
	}
	log.Printf(format, v...)
}

func NewClient() *sdk.Client {
	AuthorizationKey := viper.GetString("Authorization")

	c := sdk.NewClientWithAuth("latitudesh", " ", nil)

	if AuthorizationKey != "" {
		c = sdk.NewClientWithAuth("latitudesh", AuthorizationKey, nil)
		c.UserAgent = UserAgent
	}

	return c
}

func InitViperConfigs() {
	// look for default config
	// Find home directory
	home, err := homedir.Dir()
	cobra.CheckErr(err)

	// When running with sudo, try to use the real user's config first
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		LogDebugf("Detected sudo context. SUDO_USER=%s", sudoUser)
		// Look up the real user's home directory
		if usr, err := user.Lookup(sudoUser); err == nil {
			realHome := usr.HomeDir
			configPath := path.Join(realHome, ".config", ExeName)
			viper.AddConfigPath(configPath)
			LogDebugf("Added sudo user config path: %s", configPath)
		} else {
			LogDebugf("Could not lookup user %s: %v", sudoUser, err)
		}
	}

	// Also check current home directory (works for both sudo and non-sudo)
	viper.AddConfigPath(path.Join(home, ".config", ExeName))
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		LogDebugf("Error: loading config file: %v", err)
		return
	}
	LogDebugf("Using config file: %v", viper.ConfigFileUsed())
}
