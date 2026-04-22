package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/config"
	"github.com/spf13/cobra"
)

var (
	configFile string
	cfg        *config.Config
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&configFile, "config", "",
		`config file path (default: ~/.config/envsecrets.toml, or $ENVSECRETS_CONFIG)`,
	)
	for _, m := range config.GlobalFields() {
		rootCmd.PersistentFlags().String(m.Flag, "", m.Usage)
	}

	rootCmd.AddCommand(versionCmd)
}

// initConfig loads configuration from file and environment variables, then
// applies any CLI flag overrides on top. Called by cobra.OnInitialize before
// any command's RunE.
func initConfig() {
	cfg = config.Load(configFile)

	// Apply CLI flag overrides — highest priority, beats env vars and config file.
	// Each FieldMeta knows its flag name and scope, so we can look up the flag
	// on the correct FlagSet and call ApplyFlag when the user explicitly passed it.
	for _, m := range config.AllFields() {
		switch m.Scope {
		case "global":
			if f := rootCmd.PersistentFlags().Lookup(m.Flag); f != nil && f.Changed {
				config.ApplyFlag(cfg, m.Key, f.Value.String())
			}
		case "gen-env":
			if f := genEnvCmd.Flags().Lookup(m.Flag); f != nil && f.Changed {
				config.ApplyFlag(cfg, m.Key, f.Value.String())
			}
		}
	}

	// Resolve computed defaults that depend on other config values.
	// kpxc_db is a stem name like vault; default to the vault name so that
	// config show displays the effective stem rather than an empty string.
	if cfg.KpxcDB == "" {
		cfg.KpxcDB = cfg.Vault
	}
}
