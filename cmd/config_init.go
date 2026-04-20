package cmd

import (
	"fmt"
	"os"

	"github.com/javorszky/envsecrets/internal/config"
	"github.com/spf13/cobra"
)

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a config file with defaults and documentation",
	Long: `Write the envsecrets config file populated with default values and
inline documentation. Exits with an error if the file already exists.

The file is written to ~/.config/envsecrets.toml by default.
Override the location with --config or $ENVSECRETS_CONFIG.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := cfg.FilePath

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists: %s — delete it first or edit it manually", path)
		}

		if err := os.WriteFile(path, []byte(config.GenerateConfigTemplate()), 0o600); err != nil {
			return fmt.Errorf("writing config file %q: %w", path, err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "✓ config file written: %s\n", path)
		_, _ = fmt.Fprintln(os.Stdout)
		_, _ = fmt.Fprintln(os.Stdout, "💡 Tip: the default op_vault \"Envsecrets\" will be created automatically")
		_, _ = fmt.Fprintln(os.Stdout, "   on first write. Change it in the config file if you prefer a different name.")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
}
