package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/javorszky/envsecrets/internal/config"
	"github.com/spf13/cobra"
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print all configuration values and their sources",
	Long: `Print every configuration option, its current effective value, and
which sources contributed to it — default, config file, environment variable,
or CLI flag — with an emoji grid showing which source is active (🏆).`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Config file status line.
		if cfg.FileFound {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s\n\n", cfg.FilePath)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "Config file: %s (not found, using defaults)\n\n", cfg.FilePath)
		}

		// Column widths (display chars):
		//   option : 12   source columns : 10 each   value: rest
		const (
			wOption = 12
			wSource = 10
		)

		header := padRight("OPTION", wOption) + "  " +
			padRight("📦 default", wSource) + "  " +
			padRight("📄 file", wSource) + "  " +
			padRight("🌿 env", wSource) + "  " +
			padRight("🚩 flag", wSource) + "  " +
			"VALUE"
		_, _ = fmt.Fprintln(os.Stdout, header)

		for _, m := range config.AllFields() {
			state := cfg.Sources[m.Key]
			value := config.GetValue(cfg, m.Key)

			defCell := sourceCell("📦", state.Active == "default")
			fileCell := sourceCell(boolEmoji(state.FileSet), state.Active == "file")
			envCell := sourceCell(boolEmoji(state.EnvSet), state.Active == "env")
			flagCell := sourceCell(boolEmoji(state.FlagSet), state.Active == "flag")

			_, _ = fmt.Fprintf(os.Stdout, "%s  %s  %s  %s  %s  %s\n",
				padRight(m.Key, wOption),
				padRight(defCell, wSource),
				padRight(fileCell, wSource),
				padRight(envCell, wSource),
				padRight(flagCell, wSource),
				value,
			)
		}
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
}

// --- display helpers --------------------------------------------------------

// wideRunes is the set of emoji used in config show output.
// Each occupies 2 display columns in a terminal, unlike ASCII characters
// which occupy 1. fmt.Printf width verbs count bytes, not display columns,
// so we pad manually.
var wideRunes = map[rune]bool{
	'📦': true, '🏆': true, '📄': true, '🌿': true,
	'🚩': true, '✅': true, '🚫': true,
}

// displayWidth returns the number of terminal display columns occupied by s.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if wideRunes[r] {
			w += 2
		} else {
			w++
		}
	}

	return w
}

// padRight pads s with spaces on the right so its display width equals width.
// If s is already at least width display columns wide, it is returned as-is.
func padRight(s string, width int) string {
	n := displayWidth(s)
	if n < width {
		return s + strings.Repeat(" ", width-n)
	}

	return s
}

// boolEmoji returns ✅ when set is true, 🚫 when false.
func boolEmoji(set bool) string {
	if set {
		return "✅"
	}

	return "🚫"
}

// sourceCell prefixes emoji with 🏆 when winner is true, signalling that this
// source is the one currently active for the field.
func sourceCell(emoji string, winner bool) string {
	if winner {
		return "🏆" + emoji
	}

	return emoji
}
