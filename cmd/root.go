package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/config"
	"github.com/spf13/cobra"
)

var (
	configFile  string
	vaultFlag   string
	opVaultFlag string
	cfg         *config.Config
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
		`keychain file name (default: "envsecrets", or $ENVSECRETS_VAULT, or config file)`,
	)
	rootCmd.PersistentFlags().StringVar(
		&opVaultFlag, "op-vault", "",
		`1Password vault name (default: "Private", or $ENVSECRETS_OP_VAULT, or config file)`,
	)
}

// initConfig loads configuration from file and environment variables, then
// applies any CLI flag overrides on top. Called by cobra.OnInitialize before
// any command's RunE.
func initConfig() {
	cfg = config.Load(configFile)

	// Apply CLI flag overrides — highest priority, beats env vars and config file.
	// Flags for --template and --output (owned by genEnvCmd) are applied here
	// too, since initConfig runs after all flags are parsed.
	if rootCmd.PersistentFlags().Lookup("vault").Changed {
		cfg.Vault = vaultFlag
		cfg.Sources.Vault = "flag (--vault)"
	}
	if rootCmd.PersistentFlags().Lookup("op-vault").Changed {
		cfg.OpVault = opVaultFlag
		cfg.Sources.OpVault = "flag (--op-vault)"
	}
	if genEnvCmd.Flags().Lookup("template").Changed {
		cfg.Template = templateFlag
		cfg.Sources.Template = "flag (--template)"
	}
	if genEnvCmd.Flags().Lookup("output").Changed {
		cfg.Output = outputFlag
		cfg.Sources.Output = "flag (--output)"
	}
}
