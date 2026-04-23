package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull all secrets from the durable store into Keychain",
	Long: `Sync pulls every item from the configured durable store (1Password or
KeePassXC) and writes it into the local macOS Keychain.

Run this once on a new machine after cloning a project to bootstrap
your local Keychain. After this, all fetches can work fully offline.

Requires the durable store to be available (1Password app running and
unlocked, or keepassxc-cli installed for KeePassXC).

Examples:
  envsecrets sync
  envsecrets sync --vault Work`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mgr := secrets.New(cfg.Vault, cfg.OpVault, cfg.DurableBackend, cfg.KpxcDB, cfg.KsmConfig, cfg.KsmFolder)

		fmt.Fprintf(os.Stdout, "syncing secrets into Keychain...\n")

		n, err := mgr.Sync(ctx)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ synced %d secret(s) into Keychain\n", n)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
