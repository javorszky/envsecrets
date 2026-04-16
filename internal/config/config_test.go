package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javorszky/envsecrets/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTempConfig writes content to a temporary TOML file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "envsecrets.toml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// nonExistentPath returns a path inside a temp dir that will never be created.
// Passing this to Load() guarantees FileFound == false regardless of what
// exists at ~/.config/envsecrets.toml on the test machine.
func nonExistentPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "nonexistent-envsecrets.toml")
}

// ---------------------------------------------------------------------------
// TestLoad — all behaviours driven by an explicit configFlagValue
// ---------------------------------------------------------------------------

func TestLoad(t *testing.T) {
	// Not parallel: tests manipulate env vars via t.Setenv.

	tests := []struct {
		name               string
		fileContent        string            // TOML; empty → pass nonExistentPath to Load
		envVars            map[string]string // set via t.Setenv before calling Load
		wantVault          string
		wantTemplate       string
		wantOutput         string
		wantFileFound      bool
		wantSourceVault    string
		wantSourceTemplate string
		wantSourceOutput   string
	}{
		// ----------------------------------------------------------------
		// Defaults
		// ----------------------------------------------------------------
		{
			name:               "no file no env — all built-in defaults",
			wantVault:          "Private",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      false,
			wantSourceVault:    "default",
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},

		// ----------------------------------------------------------------
		// Config file
		// ----------------------------------------------------------------
		{
			name: "config file sets all three values",
			fileContent: `vault = "Work"
template = "staging.env.tpl"
output = "staging.env"
`,
			wantVault:          "Work",
			wantTemplate:       "staging.env.tpl",
			wantOutput:         "staging.env",
			wantFileFound:      true,
			wantSourceVault:    "config file",
			wantSourceTemplate: "config file",
			wantSourceOutput:   "config file",
		},
		{
			name: "config file sets vault only — template and output fall back to defaults",
			fileContent: `vault = "Personal"
`,
			wantVault:          "Personal",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      true,
			wantSourceVault:    "config file",
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},

		// ----------------------------------------------------------------
		// Environment variables
		// ----------------------------------------------------------------
		{
			name: "ENVSECRETS_VAULT overrides default",
			envVars: map[string]string{
				"ENVSECRETS_VAULT": "EnvVault",
			},
			wantVault:          "EnvVault",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      false,
			wantSourceVault:    "env ($ENVSECRETS_VAULT)",
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},
		{
			name: "all three env vars set",
			envVars: map[string]string{
				"ENVSECRETS_VAULT":    "EnvVault",
				"ENVSECRETS_TEMPLATE": "env.tpl",
				"ENVSECRETS_OUTPUT":   "env.out",
			},
			wantVault:          "EnvVault",
			wantTemplate:       "env.tpl",
			wantOutput:         "env.out",
			wantFileFound:      false,
			wantSourceVault:    "env ($ENVSECRETS_VAULT)",
			wantSourceTemplate: "env ($ENVSECRETS_TEMPLATE)",
			wantSourceOutput:   "env ($ENVSECRETS_OUTPUT)",
		},
		{
			name: "env var overrides config file value",
			fileContent: `vault = "FileVault"
`,
			envVars: map[string]string{
				"ENVSECRETS_VAULT": "EnvVault",
			},
			wantVault:          "EnvVault",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      true,
			wantSourceVault:    "env ($ENVSECRETS_VAULT)", // env wins
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},
		{
			name: "env var overrides for template and output but not vault",
			fileContent: `vault = "FileVault"
template = "file.tpl"
output = "file.out"
`,
			envVars: map[string]string{
				"ENVSECRETS_TEMPLATE": "env.tpl",
				"ENVSECRETS_OUTPUT":   "env.out",
			},
			wantVault:          "FileVault",
			wantTemplate:       "env.tpl",
			wantOutput:         "env.out",
			wantFileFound:      true,
			wantSourceVault:    "config file",
			wantSourceTemplate: "env ($ENVSECRETS_TEMPLATE)",
			wantSourceOutput:   "env ($ENVSECRETS_OUTPUT)",
		},

		// ----------------------------------------------------------------
		// FileFound / FilePath
		// ----------------------------------------------------------------
		{
			name:          "file not found — FileFound is false",
			wantFileFound: false,
			// other fields fall through to defaults
			wantVault:          "Private",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantSourceVault:    "default",
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},
		{
			name:               "file found — FileFound is true",
			fileContent:        `vault = "Exists"` + "\n",
			wantFileFound:      true,
			wantVault:          "Exists",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantSourceVault:    "config file",
			wantSourceTemplate: "default",
			wantSourceOutput:   "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure the three config env vars start unset for every sub-test.
			t.Setenv("ENVSECRETS_VAULT", "")
			t.Setenv("ENVSECRETS_TEMPLATE", "")
			t.Setenv("ENVSECRETS_OUTPUT", "")
			// Also clear ENVSECRETS_CONFIG so it doesn't interfere when we
			// pass the path explicitly as configFlagValue.
			t.Setenv("ENVSECRETS_CONFIG", "")

			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			// Choose the path to pass to Load.
			var cfgPath string
			if tc.fileContent != "" {
				cfgPath = writeTempConfig(t, tc.fileContent)
			} else {
				cfgPath = nonExistentPath(t)
			}

			cfg := config.Load(cfgPath)

			assert.Equal(t, tc.wantVault, cfg.Vault, "Vault")
			assert.Equal(t, tc.wantTemplate, cfg.Template, "Template")
			assert.Equal(t, tc.wantOutput, cfg.Output, "Output")
			assert.Equal(t, tc.wantFileFound, cfg.FileFound, "FileFound")
			assert.Equal(t, tc.wantSourceVault, cfg.Sources.Vault, "Sources.Vault")
			assert.Equal(t, tc.wantSourceTemplate, cfg.Sources.Template, "Sources.Template")
			assert.Equal(t, tc.wantSourceOutput, cfg.Sources.Output, "Sources.Output")

			// FilePath should always be non-empty.
			assert.NotEmpty(t, cfg.FilePath, "FilePath")
			if tc.wantFileFound {
				// When file was found, FilePath must equal the file we wrote.
				assert.Equal(t, cfgPath, cfg.FilePath, "FilePath when file found")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestLoad_EnvsecretsCfgEnvVar — verifies ENVSECRETS_CONFIG path resolution
// ---------------------------------------------------------------------------

func TestLoad_EnvsecretsCfgEnvVar(t *testing.T) {
	// Not parallel: manipulates ENVSECRETS_CONFIG.

	t.Setenv("ENVSECRETS_VAULT", "")
	t.Setenv("ENVSECRETS_TEMPLATE", "")
	t.Setenv("ENVSECRETS_OUTPUT", "")

	cfgPath := writeTempConfig(t, `vault = "FromEnvCfg"`+"\n")
	t.Setenv("ENVSECRETS_CONFIG", cfgPath)

	// Pass empty string so Load falls through to the env-var path.
	cfg := config.Load("")

	require.True(t, cfg.FileFound)
	assert.Equal(t, "FromEnvCfg", cfg.Vault)
	assert.Equal(t, "config file", cfg.Sources.Vault)
}
