package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/spf13/cobra"
)

var storeCmd = &cobra.Command{
	Use:   "store <key> <value>",
	Short: "Store a new secret",
	Long: `Store a secret in Keychain and 1Password.

If the key already exists in either backend, use 'update' instead.
Both backends are written; 1Password failure is non-fatal.

Examples:
  envsecrets store STRIPE_SECRET sk_live_abc123
  envsecrets store --vault Work DB_PASSWORD hunter2`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		mgr := secrets.New(vaultFlag)

		if err := mgr.Set(key, value); err != nil {
			return fmt.Errorf("store failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ stored %q\n", key)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(storeCmd)
}
