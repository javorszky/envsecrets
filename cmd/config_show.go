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
or CLI flag — with an emoji grid showing which source is active (🏆).

With --verbose / -v: expand each option into a full block showing the value
(✅) or "(not set)" (🚫) for every source, and annotate lower-priority sources
with "⬆️ superseded by ..." so it is immediately clear why the effective value
is what it is.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Config file status line (shared by both modes).
		if cfg.FileFound {
			fmt.Fprintf(os.Stdout, "Config file: %s\n\n", cfg.FilePath)
		} else {
			fmt.Fprintf(os.Stdout, "Config file: %s (not found, using defaults)\n\n", cfg.FilePath)
		}

		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			verboseOutput()
			return
		}

		// Compact grid (default).
		const wSource = 10

		// Compute column width from the longest key name so new fields never
		// cause misalignment.
		wOption := len("OPTION") // minimum: at least as wide as the header
		for _, m := range config.AllFields() {
			if len(m.Key) > wOption {
				wOption = len(m.Key)
			}
		}

		header := padRight("OPTION", wOption) + "  " +
			padRight("📦 default", wSource) + "  " +
			padRight("📄 file", wSource) + "  " +
			padRight("🌿 env", wSource) + "  " +
			padRight("🚩 flag", wSource) + "  " +
			"VALUE"
		fmt.Fprintln(os.Stdout, header)

		for _, m := range config.AllFields() {
			state := cfg.Sources[m.Key]
			value := config.GetValue(cfg, m)

			defCell := sourceCell("📦", state.Active == config.ActiveDefault)
			fileCell := sourceCell(boolEmoji(state.Flags.Has(config.SourceFile)), state.Active == config.ActiveFile)
			envCell := sourceCell(boolEmoji(state.Flags.Has(config.SourceEnv)), state.Active == config.ActiveEnv)
			flagCell := sourceCell(boolEmoji(state.Flags.Has(config.SourceFlag)), state.Active == config.ActiveFlag)

			fmt.Fprintf(os.Stdout, "%s  %s  %s  %s  %s  %s\n",
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
	configShowCmd.Flags().BoolP("verbose", "v", false, "show full per-source details with supersession annotations")
}

// --- verbose output ---------------------------------------------------------

// srcRow holds one source line's display data for verbose config output.
// Declared at package level so verboseOutput and supersededBy share the type.
type srcRow struct {
	emoji     string
	label     string
	source    config.ActiveSource // which ActiveSource constant this row represents
	isSet     bool
	identVal  string // shown when isSet is true  (e.g. `✅  vault = "myproject"`)
	notSetVal string // shown when isSet is false (e.g. `🚫  ENVSECRETS_VAULT (not set)`)
}

// verboseOutput prints a full per-option block for every config field.
// For each source it shows ✅ + value or 🚫 + "(not set)", and annotates
// set-but-losing sources with "⬆️ superseded by <next higher set source>".
func verboseOutput() {
	const (
		wLabel = 12 // "config file" is 11 display chars; pad to 12
		wIdent = 46 // wide enough for "✅  ENVSECRETS_OP_VAULT = \"some-value\""
	)

	sep := strings.Repeat("─", 66)

	for _, m := range config.AllFields() {
		state := cfg.Sources[m.Key]

		fmt.Fprintln(os.Stdout, sep)
		fmt.Fprintln(os.Stdout, m.Key)

		rows := []srcRow{
			{
				emoji:    "📦",
				label:    "default",
				source:   config.ActiveDefault,
				isSet:    true,
				identVal: "✅  " + fmt.Sprintf("%q", m.Default),
			},
			{
				emoji:     "📄",
				label:     "config file",
				source:    config.ActiveFile,
				isSet:     state.Flags.Has(config.SourceFile),
				identVal:  "✅  " + fmt.Sprintf("%s = %q", m.Key, state.FileValue),
				notSetVal: "🚫  (not set)",
			},
			{
				emoji:     "🌿",
				label:     "env var",
				source:    config.ActiveEnv,
				isSet:     state.Flags.Has(config.SourceEnv),
				identVal:  "✅  " + fmt.Sprintf("%s = %q", m.EnvVar, state.EnvValue),
				notSetVal: "🚫  " + m.EnvVar + " (not set)",
			},
			{
				emoji:     "🚩",
				label:     "cli flag",
				source:    config.ActiveFlag,
				isSet:     state.Flags.Has(config.SourceFlag),
				identVal:  "✅  " + fmt.Sprintf("--%s = %q", m.Flag, state.FlagValue),
				notSetVal: "🚫  --" + m.Flag + " (not set)",
			},
		}

		for i, row := range rows {
			labelCell := padRight(row.label, wLabel)

			if !row.isSet {
				fmt.Fprintf(os.Stdout, "  %s  %s  %s\n",
					row.emoji,
					labelCell,
					row.notSetVal,
				)
				continue
			}

			identCell := padRight(row.identVal, wIdent)
			status := supersededBy(rows, i, m)
			if status == "" {
				status = "🏆 ← active"
			}

			fmt.Fprintf(os.Stdout, "  %s  %s  %s  %s\n",
				row.emoji,
				labelCell,
				identCell,
				status,
			)
		}

		fmt.Fprintln(os.Stdout)
	}

	fmt.Fprintln(os.Stdout, sep)
}

// supersededBy returns "⬆️ superseded by <label> (<identifier>)" if any
// higher-priority set source exists above index i in rows, otherwise "".
func supersededBy(rows []srcRow, i int, m config.FieldMeta) string {
	for j := i + 1; j < len(rows); j++ {
		if !rows[j].isSet {
			continue
		}

		switch rows[j].source {
		case config.ActiveFile:
			return "⬆️ superseded by config file"
		case config.ActiveEnv:
			return fmt.Sprintf("⬆️ superseded by env var (%s)", m.EnvVar)
		case config.ActiveFlag:
			return fmt.Sprintf("⬆️ superseded by cli flag (--%s)", m.Flag)
		}
	}

	return ""
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
