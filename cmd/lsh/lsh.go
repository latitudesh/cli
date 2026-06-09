// Package to configure options that should be acessible througout all commands
package lsh

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"

	latitudeshgosdk "github.com/latitudesh/latitudesh-go-sdk"
	"github.com/latitudesh/lsh/internal/config"
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
	} else {
		LogDebugf("[CONFIG] ✓ Using config file: %v\n", viper.ConfigFileUsed())
	}

	// Hydrate the legacy top-level `Authorization` / `api-version` viper
	// keys (which the generated commands read) from the active profile.
	// This keeps every existing operation working unchanged while the
	// new login flow stores credentials per profile.
	_ = HydrateFromActiveProfile("")
}

// HydrateFromActiveProfile resolves the active profile (honoring
// LATITUDESH_TOKEN, the explicit override, LSH_PROFILE and the stored
// default_profile, in that order) and sets the per-request viper keys
// used by the generated SDK calls. It is safe to call multiple times —
// useful when a `--profile` flag is parsed after the initial load.
func HydrateFromActiveProfile(override string) error {
	if token := os.Getenv("LATITUDESH_TOKEN"); token != "" {
		viper.Set("Authorization", token)
		if viper.GetString("api-version") == "" {
			viper.Set("api-version", "2023-06-01")
		}
		LogDebugf("[AUTH] Using LATITUDESH_TOKEN from environment")
		if override != "" {
			fmt.Fprintf(os.Stderr, "warning: --profile %q ignored because LATITUDESH_TOKEN is set\n", override)
		}
		return nil
	}

	f, err := config.Load()
	if err != nil {
		if override != "" {
			return fmt.Errorf("could not load profile config: %w", err)
		}
		LogDebugf("[CONFIG] Could not load profile config: %v", err)
		return nil
	}

	_, profile, err := f.Resolve(override)
	if err != nil {
		// An explicit --profile that can't be resolved must fail loudly:
		// silently falling back to the default profile would run the
		// command under the wrong team's credentials.
		if override != "" {
			return fmt.Errorf("profile %q not found — run `lsh profile list` to see available profiles", override)
		}
		if !errors.Is(err, config.ErrProfileNotFound) {
			LogDebugf("[CONFIG] Could not resolve profile: %v", err)
		}
		return nil
	}

	if profile.Authorization != "" {
		viper.Set("Authorization", profile.Authorization)
	}
	if profile.APIVersion != "" {
		viper.Set("api-version", profile.APIVersion)
	}
	return nil
}
