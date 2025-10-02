// Package to configure options that should be acessible througout all commands
package lsh

import (
	"fmt"
	"log"
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
	// Find home directory.
	home, err := homedir.Dir()
	cobra.CheckErr(err)

	// Search config in home directory with name ".cobra" (without extension).
	viper.AddConfigPath(path.Join(home, ".config", ExeName))
	viper.SetConfigName("config")

	if err := viper.ReadInConfig(); err != nil {
		LogDebugf("Error: loading config file: %v", err)
		return
	}
	LogDebugf("Using config file: %v", viper.ConfigFileUsed())
}
