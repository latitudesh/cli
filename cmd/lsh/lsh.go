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

	// Always log this regardless of debug flag since it's critical for troubleshooting
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser != "" {
		log.Printf("[SUDO] Detected sudo context. SUDO_USER=%s\n", sudoUser)
		// Look up the real user's home directory
		if usr, err := user.Lookup(sudoUser); err == nil {
			realHome := usr.HomeDir
			configPath := path.Join(realHome, ".config", ExeName)
			viper.AddConfigPath(configPath)
			log.Printf("[SUDO] Added sudo user config path: %s\n", configPath)
		} else {
			log.Printf("[SUDO] Could not lookup user %s: %v\n", sudoUser, err)
		}
	} else {
		log.Printf("[CONFIG] Running as normal user (no sudo)\n")
	}

	// Also check current home directory (works for both sudo and non-sudo)
	currentConfigPath := path.Join(home, ".config", ExeName)
	viper.AddConfigPath(currentConfigPath)
	log.Printf("[CONFIG] Added current home config path: %s\n", currentConfigPath)
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[CONFIG] Error loading config file: %v\n", err)
		log.Printf("[CONFIG] Searched in paths: %s/.config/%s\n", home, ExeName)
		if sudoUser != "" {
			log.Printf("[CONFIG] Also searched sudo user paths\n")
		}
		return
	}
	log.Printf("[CONFIG] ✓ Using config file: %v\n", viper.ConfigFileUsed())
}
