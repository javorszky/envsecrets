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
	Short: "Manage env secrets across macOS Keychain and a durable store",
	Long: `envsecrets stores, retrieves, updates, and deletes secrets using
macOS Keychain as the primary (always-local) backend and a configurable
durable store (1Password or KeePassXC) as the sync layer.

Reads hit Keychain first. On a miss, the durable store is consulted and
the result is cached back into Keychain. Writes go to both; durable store
failure is a warning, not an error, so offline workflows continue
uninterrupted.`,
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

	// Validate fields that are used as file-name stems. Exit immediately with a
	// clear message rather than failing later with a confusing path error.
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "envsecrets: invalid configuration:", err)
		os.Exit(1)
	}
}
