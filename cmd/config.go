package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage envsecrets configuration",
	Long:  `Commands for initialising and inspecting the envsecrets configuration file.`,
}

func init() {
	rootCmd.AddCommand(configCmd)
}
