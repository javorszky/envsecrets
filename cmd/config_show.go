package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print all configuration values and their sources",
	Long: `Print every configuration option, its current effective value, and
where that value is coming from (flag, environment variable, config file, or default).`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Config file status line.
		if cfg.FileFound {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s\n\n", cfg.FilePath)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s (not found, using defaults)\n\n", cfg.FilePath)
		}

		_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n", "OPTION", "VALUE", "SOURCE")
		_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n", "------", "-----", "------")

		rows := []struct{ key, value, source string }{
			{"vault", cfg.Vault, cfg.Sources.Vault},
			{"template", cfg.Template, cfg.Sources.Template},
			{"output", cfg.Output, cfg.Sources.Output},
		}
		for _, r := range rows {
			_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n", r.key, r.value, r.source)
		}
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
