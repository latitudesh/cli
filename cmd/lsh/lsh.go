// Package to configure options that should be acessible througout all commands
package lsh

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
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

func NewClient() *latitudeshgosdk.Latitudesh {
	AuthorizationKey := viper.GetString("Authorization")

	return latitudeshgosdk.New(
		latitudeshgosdk.WithSecurity(AuthorizationKey),
	)
}

func NewContext() context.Context {
	return context.Background()
}

func InitViperConfigs() {
	// look for default config
	// Find home directory
	home, err := homedir.Dir()
	cobra.CheckErr(err)

	// Only log in debug mode
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser != "" {
		LogDebugf("[SUDO] Detected sudo context. SUDO_USER=%s\n", sudoUser)
		// Look up the real user's home directory
		if usr, err := user.Lookup(sudoUser); err == nil {
			realHome := usr.HomeDir
			configPath := path.Join(realHome, ".config", ExeName)
			viper.AddConfigPath(configPath)
			LogDebugf("[SUDO] Added sudo user config path: %s\n", configPath)
		} else {
			LogDebugf("[SUDO] Could not lookup user %s: %v\n", sudoUser, err)
		}
	} else {
		LogDebugf("[CONFIG] Running as normal user (no sudo)\n")
	}

	// Also check current home directory (works for both sudo and non-sudo)
	currentConfigPath := path.Join(home, ".config", ExeName)
	viper.AddConfigPath(currentConfigPath)
	LogDebugf("[CONFIG] Added current home config path: %s\n", currentConfigPath)
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		LogDebugf("[CONFIG] Error loading config file: %v\n", err)
		LogDebugf("[CONFIG] Searched in paths: %s/.config/%s\n", home, ExeName)
		if sudoUser != "" {
			LogDebugf("[CONFIG] Also searched sudo user paths\n")
		}
		return
	}
	LogDebugf("[CONFIG] ✓ Using config file: %v\n", viper.ConfigFileUsed())
}
