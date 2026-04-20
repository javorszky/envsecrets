package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version variables are set at build time by GoReleaser via -ldflags:
//
//	-X github.com/javorszky/envsecrets/cmd.Version={{.Version}}
//	-X github.com/javorszky/envsecrets/cmd.Commit={{.Commit}}
//	-X github.com/javorszky/envsecrets/cmd.Date={{.Date}}
//	-X github.com/javorszky/envsecrets/cmd.BuiltBy=goreleaser
//
// Defaults apply when built with plain `go build .`.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "manual"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("envsecrets %s\n  commit:   %s\n  built:    %s\n  built by: %s\n",
			Version, Commit, Date, BuiltBy)
	},
}
