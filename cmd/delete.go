package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/spf13/cobra"
)

var (
	forceFlag bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret from both backends",
	Long: `Delete a secret from Keychain and the configured durable store.

Prompts for confirmation unless --force is passed. Errors from each
backend are reported independently; a miss in one does not prevent
deletion from the other.

Examples:
  envsecrets delete STRIPE_SECRET
  envsecrets delete --force OLD_API_KEY
  envsecrets delete --vault Work DB_PASSWORD`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		key := args[0]

		if !forceFlag {
			fmt.Fprintf(os.Stdout, "Delete %q from Keychain and the durable store? [y/N] ", key)

			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()

			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Fprintln(os.Stdout, "aborted")
				return nil
			}
		}

		mgr := secrets.New(cfg.Vault, cfg.OpVault, cfg.DurableBackend, cfg.KpxcDB, cfg.KsmConfig, cfg.KsmFolder)

		if err := mgr.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ deleted %q\n", key)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	deleteCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation prompt")
}
