package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print all configuration values and their sources",
	Long: `Print every configuration option, its current effective value, and
where that value is coming from (flag, environment variable, config file, or default).`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Config file status line.
		if viper.ConfigFileUsed() != "" {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s\n\n", viper.ConfigFileUsed())
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s (not found, using defaults)\n\n", configFilePath())
		}

		_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n", "OPTION", "VALUE", "SOURCE")
		_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n", "------", "-----", "------")

		rows := []struct{ key, envVar, flagName string }{
			{"vault", "ENVSECRETS_VAULT", "vault"},
			{"template", "ENVSECRETS_TEMPLATE", "template"},
			{"output", "ENVSECRETS_OUTPUT", "output"},
		}
		for _, r := range rows {
			_, _ = fmt.Fprintf(os.Stdout, "%-12s  %-24s  %s\n",
				r.key,
				viper.GetString(r.key),
				sourceOf(r.key, r.envVar, r.flagName),
			)
		}
	},
}

// sourceOf returns a human-readable string describing where a config value
// comes from. Priority mirrors viper: flag > env > config file > default.
func sourceOf(key, envVar, flagName string) string {
	// Flag explicitly passed on the CLI?
	if f := rootCmd.PersistentFlags().Lookup(flagName); f != nil && f.Changed {
		return "flag (--" + flagName + ")"
	}
	if f := genEnvCmd.Flags().Lookup(flagName); f != nil && f.Changed {
		return "flag (--" + flagName + ")"
	}

	// Environment variable set?
	if os.Getenv(envVar) != "" {
		return "env ($" + envVar + ")"
	}

	// Explicitly present in the loaded config file?
	if configFileKeys[key] {
		return "config file"
	}

	return "default"
}

func init() {
	configCmd.AddCommand(configShowCmd)
}
