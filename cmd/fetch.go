package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch <key>",
	Short: "Fetch a secret",
	Long: `Fetch a secret value, printing it to stdout.

Keychain is tried first. On a miss, 1Password is consulted and the
result is written back into Keychain for future offline use.

The value is printed to stdout with no trailing newline, making it
safe to use in shell substitution:

  export DB_PASSWORD=$(envsecrets fetch DB_PASSWORD)

Examples:
  envsecrets fetch STRIPE_SECRET
  envsecrets fetch --vault Work DB_PASSWORD`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		key := args[0]
		mgr := secrets.New(cfg.Vault, cfg.OpVault)

		val, err := mgr.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("fetch failed: %w", err)
		}

		// No trailing newline — friendly for $() substitution.
		_, _ = fmt.Fprint(os.Stdout, val)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}
