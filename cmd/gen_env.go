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
	templateFlag string
	outputFlag   string
)

var genEnvCmd = &cobra.Command{
	Use:   "gen-env",
	Short: "Generate a .env file from a committed template",
	Long: `gen-env reads a template file where values are secret key references
(prefixed with "secret:") and writes a resolved .env file with real values.

Template format (.env.tpl — safe to commit):

  DATABASE_URL=secret:myproject_DATABASE_URL
  API_KEY=secret:myproject_API_KEY
  APP_ENV=production
  LOG_LEVEL=info

Lines without the "secret:" prefix are copied verbatim. Blank lines and
comments (#) are preserved.

The output file should be gitignored.

Examples:
  envsecrets gen-env
  envsecrets gen-env --template .env.tpl --output .env
  envsecrets gen-env --vault Work --template staging.env.tpl --output staging.env`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mgr := secrets.New(cfg.Vault)

		templatePath := cfg.Template
		tpl, err := os.Open(templatePath)
		if err != nil {
			return fmt.Errorf("opening template %q: %w", templatePath, err)
		}
		defer func() { _ = tpl.Close() }()

		outputPath := cfg.Output
		out, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("creating output %q: %w", outputPath, err)
		}
		defer func() { _ = out.Close() }()

		w := bufio.NewWriter(out)
		scanner := bufio.NewScanner(tpl)
		lineNo := 0
		resolved := 0

		for scanner.Scan() {
			lineNo++
			line := scanner.Text()

			// Preserve blank lines and comments unchanged.
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				_, _ = fmt.Fprintln(w, line)
				continue
			}

			// Split on first `=` only.
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				_, _ = fmt.Fprintln(w, line)
				continue
			}

			envKey, envVal := parts[0], parts[1]

			if !strings.HasPrefix(envVal, "secret:") {
				// Plain value — copy verbatim.
				_, _ = fmt.Fprintln(w, line)
				continue
			}

			secretKey := strings.TrimPrefix(envVal, "secret:")

			val, fetchErr := mgr.Get(ctx, secretKey)
			if fetchErr != nil {
				return fmt.Errorf("line %d: resolving %q: %w", lineNo, secretKey, fetchErr)
			}

			_, _ = fmt.Fprintf(w, "%s=%s\n", envKey, val)
			resolved++
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading template: %w", err)
		}

		if err := w.Flush(); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "✓ wrote %q (%d secret(s) resolved)\n", outputPath, resolved)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(genEnvCmd)

	genEnvCmd.Flags().StringVar(&templateFlag, "template", "", `template file path (default: ".env.tpl", or $ENVSECRETS_TEMPLATE, or config file)`)
	genEnvCmd.Flags().StringVar(&outputFlag, "output", "", `output file path (default: ".env", or $ENVSECRETS_OUTPUT, or config file)`)
}
