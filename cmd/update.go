package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <key> <value>",
	Short: "Update an existing secret",
	Long: `Update a secret in Keychain and 1Password.

Semantically equivalent to 'store' — both upsert — but signals intent
that you expect the key to already exist.

Examples:
  envsecrets update STRIPE_SECRET sk_live_newvalue
  envsecrets update --vault Work DB_PASSWORD newpassword`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		mgr := secrets.New(vaultFlag)

		if err := mgr.Update(key, value); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ updated %q\n", key)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
