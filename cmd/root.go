package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	vaultFlag string
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
	rootCmd.PersistentFlags().StringVar(
		&vaultFlag,
		"vault",
		defaultVault(),
		`1Password vault name (overrides ENVSECRETS_VAULT env var)`,
	)
}

// defaultVault reads the vault name from the environment, falling back to
// "Private" which is the default personal vault name in 1Password.
func defaultVault() string {
	if v := os.Getenv("ENVSECRETS_VAULT"); v != "" {
		return v
	}

	return "Private"
}
