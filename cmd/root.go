package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile     string
	vaultFlag      string
	configFileKeys map[string]bool // keys explicitly present in the loaded config file
)

var rootCmd = &cobra.Command{
	Use:   "envsecrets",
	Short: "Manage env secrets across macOS Keychain and 1Password",
	Long: `envsecrets stores, retrieves, updates, and deletes secrets using
macOS Keychain as the primary (always-local) backend and 1Password
as a durable sync layer.

Reads hit Keychain first. On a miss, 1Password is tried and the result
is cached back into Keychain. Writes go to both; 1Password failure is a
warning, not an error, so offline workflows continue uninterrupted.`,
}

// Execute is called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&configFile, "config", "",
		`config file path (default: ~/.config/envsecrets.toml, or $ENVSECRETS_CONFIG)`,
	)
	rootCmd.PersistentFlags().StringVar(
		&vaultFlag, "vault", "",
		`1Password vault name (default: "Private", or $ENVSECRETS_VAULT, or config file)`,
	)

	_ = viper.BindPFlag("vault", rootCmd.PersistentFlags().Lookup("vault"))
	_ = viper.BindEnv("vault", "ENVSECRETS_VAULT")
	viper.SetDefault("vault", "Private")
}

// initConfig loads the config file and records which keys came from it.
// Called by cobra.OnInitialize before any command's RunE.
func initConfig() {
	// Determine config file location:
	// priority: --config flag > ENVSECRETS_CONFIG env var > default path
	switch {
	case configFile != "":
		viper.SetConfigFile(configFile)
	case os.Getenv("ENVSECRETS_CONFIG") != "":
		viper.SetConfigFile(os.Getenv("ENVSECRETS_CONFIG"))
	default:
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".config"))
		}
		viper.SetConfigName("envsecrets")
		viper.SetConfigType("toml")
	}

	// Read config file. Ignore error — file may not exist yet.
	if err := viper.ReadInConfig(); err == nil {
		// Record which keys were explicitly set in the config file.
		// Use a second viper instance (no env/defaults) so AllKeys() only
		// returns what is literally present in the file.
		fv := viper.New()
		fv.SetConfigFile(viper.ConfigFileUsed())
		if fv.ReadInConfig() == nil {
			configFileKeys = make(map[string]bool)
			for _, k := range fv.AllKeys() {
				configFileKeys[k] = true
			}
		}
	}
}

// configFilePath returns the resolved config file path, regardless of
// whether the file exists. Used by config show and config init.
func configFilePath() string {
	if configFile != "" {
		return configFile
	}
	if v := os.Getenv("ENVSECRETS_CONFIG"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/envsecrets.toml"
	}
	return filepath.Join(home, ".config", "envsecrets.toml")
}
