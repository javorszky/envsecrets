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
		wantOpVault        string
		wantTemplate       string
		wantOutput         string
		wantFileFound      bool
		wantSourceVault    config.ActiveSource
		wantSourceOpVault  config.ActiveSource
		wantSourceTemplate config.ActiveSource
		wantSourceOutput   config.ActiveSource
	}{
		// ----------------------------------------------------------------
		// Defaults
		// ----------------------------------------------------------------
		{
			name:               "no file no env — all built-in defaults",
			wantVault:          "envsecrets",
			wantOpVault:        "Envsecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      false,
			wantSourceVault:    config.ActiveDefault,
			wantSourceOpVault:  config.ActiveDefault,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},

		// ----------------------------------------------------------------
		// Config file
		// ----------------------------------------------------------------
		{
			name: "config file sets all values",
			fileContent: `vault = "work"
op_vault = "Work"
template = "staging.env.tpl"
output = "staging.env"
`,
			wantVault:          "work",
			wantOpVault:        "Work",
			wantTemplate:       "staging.env.tpl",
			wantOutput:         "staging.env",
			wantFileFound:      true,
			wantSourceVault:    config.ActiveFile,
			wantSourceOpVault:  config.ActiveFile,
			wantSourceTemplate: config.ActiveFile,
			wantSourceOutput:   config.ActiveFile,
		},
		{
			name: "config file sets vault only — op_vault template and output fall back to defaults",
			fileContent: `vault = "myproject"
`,
			wantVault:          "myproject",
			wantOpVault:        "Envsecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      true,
			wantSourceVault:    config.ActiveFile,
			wantSourceOpVault:  config.ActiveDefault,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
		{
			name: "config file sets op_vault only",
			fileContent: `op_vault = "envsecrets"
`,
			wantVault:          "envsecrets",
			wantOpVault:        "envsecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      true,
			wantSourceVault:    config.ActiveDefault,
			wantSourceOpVault:  config.ActiveFile,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},

		// ----------------------------------------------------------------
		// Environment variables
		// ----------------------------------------------------------------
		{
			name: "ENVSECRETS_VAULT overrides default",
			envVars: map[string]string{
				"ENVSECRETS_VAULT": "envkc",
			},
			wantVault:          "envkc",
			wantOpVault:        "Envsecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      false,
			wantSourceVault:    config.ActiveEnv,
			wantSourceOpVault:  config.ActiveDefault,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
		{
			name: "ENVSECRETS_OP_VAULT overrides default",
			envVars: map[string]string{
				"ENVSECRETS_OP_VAULT": "MySecrets",
			},
			wantVault:          "envsecrets",
			wantOpVault:        "MySecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      false,
			wantSourceVault:    config.ActiveDefault,
			wantSourceOpVault:  config.ActiveEnv,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
		{
			name: "all env vars set",
			envVars: map[string]string{
				"ENVSECRETS_VAULT":    "envkc",
				"ENVSECRETS_OP_VAULT": "EnvOP",
				"ENVSECRETS_TEMPLATE": "env.tpl",
				"ENVSECRETS_OUTPUT":   "env.out",
			},
			wantVault:          "envkc",
			wantOpVault:        "EnvOP",
			wantTemplate:       "env.tpl",
			wantOutput:         "env.out",
			wantFileFound:      false,
			wantSourceVault:    config.ActiveEnv,
			wantSourceOpVault:  config.ActiveEnv,
			wantSourceTemplate: config.ActiveEnv,
			wantSourceOutput:   config.ActiveEnv,
		},
		{
			name: "env vars override config file values",
			fileContent: `vault = "filekc"
op_vault = "FileOP"
`,
			envVars: map[string]string{
				"ENVSECRETS_VAULT":    "envkc",
				"ENVSECRETS_OP_VAULT": "EnvOP",
			},
			wantVault:          "envkc",
			wantOpVault:        "EnvOP",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantFileFound:      true,
			wantSourceVault:    config.ActiveEnv, // env wins
			wantSourceOpVault:  config.ActiveEnv, // env wins
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
		{
			name: "env var overrides for template and output but not vault or op_vault",
			fileContent: `vault = "filekc"
op_vault = "FileOP"
template = "file.tpl"
output = "file.out"
`,
			envVars: map[string]string{
				"ENVSECRETS_TEMPLATE": "env.tpl",
				"ENVSECRETS_OUTPUT":   "env.out",
			},
			wantVault:          "filekc",
			wantOpVault:        "FileOP",
			wantTemplate:       "env.tpl",
			wantOutput:         "env.out",
			wantFileFound:      true,
			wantSourceVault:    config.ActiveFile,
			wantSourceOpVault:  config.ActiveFile,
			wantSourceTemplate: config.ActiveEnv,
			wantSourceOutput:   config.ActiveEnv,
		},

		// ----------------------------------------------------------------
		// FileFound / FilePath
		// ----------------------------------------------------------------
		{
			name:          "file not found — FileFound is false",
			wantFileFound: false,
			// other fields fall through to defaults
			wantVault:          "envsecrets",
			wantOpVault:        "Envsecrets",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantSourceVault:    config.ActiveDefault,
			wantSourceOpVault:  config.ActiveDefault,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
		{
			name:               "file found — FileFound is true",
			fileContent:        `op_vault = "Exists"` + "\n",
			wantFileFound:      true,
			wantVault:          "envsecrets",
			wantOpVault:        "Exists",
			wantTemplate:       ".env.tpl",
			wantOutput:         ".env",
			wantSourceVault:    config.ActiveDefault,
			wantSourceOpVault:  config.ActiveFile,
			wantSourceTemplate: config.ActiveDefault,
			wantSourceOutput:   config.ActiveDefault,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Ensure the config env vars start unset for every sub-test.
			t.Setenv("ENVSECRETS_VAULT", "")
			t.Setenv("ENVSECRETS_OP_VAULT", "")
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
			assert.Equal(t, tc.wantOpVault, cfg.OpVault, "OpVault")
			assert.Equal(t, tc.wantTemplate, cfg.Template, "Template")
			assert.Equal(t, tc.wantOutput, cfg.Output, "Output")
			assert.Equal(t, tc.wantFileFound, cfg.FileFound, "FileFound")
			assert.Equal(t, tc.wantSourceVault, cfg.Sources["vault"].Active, "Sources[vault].Active")
			assert.Equal(t, tc.wantSourceOpVault, cfg.Sources["op_vault"].Active, "Sources[op_vault].Active")
			assert.Equal(t, tc.wantSourceTemplate, cfg.Sources["template"].Active, "Sources[template].Active")
			assert.Equal(t, tc.wantSourceOutput, cfg.Sources["output"].Active, "Sources[output].Active")

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
	t.Setenv("ENVSECRETS_OP_VAULT", "")
	t.Setenv("ENVSECRETS_TEMPLATE", "")
	t.Setenv("ENVSECRETS_OUTPUT", "")

	cfgPath := writeTempConfig(t, `op_vault = "FromEnvCfg"`+"\n")
	t.Setenv("ENVSECRETS_CONFIG", cfgPath)

	// Pass empty string so Load falls through to the env-var path.
	cfg := config.Load("")

	require.True(t, cfg.FileFound)
	assert.Equal(t, "FromEnvCfg", cfg.OpVault)
	assert.Equal(t, config.ActiveFile, cfg.Sources["op_vault"].Active)
}

// ---------------------------------------------------------------------------
// TestLoad_SourceValues — FileValue and EnvValue are captured in SourceState
// ---------------------------------------------------------------------------

func TestLoad_SourceValues(t *testing.T) {
	// Not parallel: manipulates env vars.
	t.Setenv("ENVSECRETS_VAULT", "")
	t.Setenv("ENVSECRETS_OP_VAULT", "")
	t.Setenv("ENVSECRETS_TEMPLATE", "")
	t.Setenv("ENVSECRETS_OUTPUT", "")
	t.Setenv("ENVSECRETS_CONFIG", "")

	cfgPath := writeTempConfig(t, `vault = "filekc"`+"\n")
	t.Setenv("ENVSECRETS_OP_VAULT", "EnvOP")

	cfg := config.Load(cfgPath)

	// vault was set in the config file.
	assert.True(t, cfg.Sources["vault"].Flags.Has(config.SourceFile))
	assert.Equal(t, "filekc", cfg.Sources["vault"].FileValue)

	// op_vault was set via env var.
	assert.True(t, cfg.Sources["op_vault"].Flags.Has(config.SourceEnv))
	assert.Equal(t, "EnvOP", cfg.Sources["op_vault"].EnvValue)

	// template was not set by file or env — values must be empty.
	assert.False(t, cfg.Sources["template"].Flags.Has(config.SourceFile))
	assert.False(t, cfg.Sources["template"].Flags.Has(config.SourceEnv))
	assert.Empty(t, cfg.Sources["template"].FileValue)
	assert.Empty(t, cfg.Sources["template"].EnvValue)
}

// ---------------------------------------------------------------------------
// TestAllFields — verifies struct tag reflection
// ---------------------------------------------------------------------------

func TestAllFields(t *testing.T) {
	t.Parallel()

	fields := config.AllFields()

	require.Len(t, fields, 8)

	// All eight TOML keys must be present in order.
	assert.Equal(t, "vault", fields[0].Key)
	assert.Equal(t, "op_vault", fields[1].Key)
	assert.Equal(t, "durable_backend", fields[2].Key)
	assert.Equal(t, "kpxc_db", fields[3].Key)
	assert.Equal(t, "ksm_config", fields[4].Key)
	assert.Equal(t, "ksm_folder", fields[5].Key)
	assert.Equal(t, "template", fields[6].Key)
	assert.Equal(t, "output", fields[7].Key)

	// vault — global scope
	vault := fields[0]
	assert.Equal(t, "ENVSECRETS_VAULT", vault.EnvVar)
	assert.Equal(t, "vault", vault.Flag)
	assert.Equal(t, "envsecrets", vault.Default)
	assert.Equal(t, "global", vault.Scope)
	assert.NotEmpty(t, vault.Usage)

	// op_vault — global scope
	opVault := fields[1]
	assert.Equal(t, "ENVSECRETS_OP_VAULT", opVault.EnvVar)
	assert.Equal(t, "op-vault", opVault.Flag)
	assert.Equal(t, "Envsecrets", opVault.Default)
	assert.Equal(t, "global", opVault.Scope)
	assert.NotEmpty(t, opVault.Usage)

	// durable_backend — global scope
	durableBackend := fields[2]
	assert.Equal(t, "ENVSECRETS_DURABLE_BACKEND", durableBackend.EnvVar)
	assert.Equal(t, "durable-backend", durableBackend.Flag)
	assert.Equal(t, "1password", durableBackend.Default)
	assert.Equal(t, "global", durableBackend.Scope)
	assert.NotEmpty(t, durableBackend.Usage)

	// kpxc_db — global scope
	kpxcDB := fields[3]
	assert.Equal(t, "ENVSECRETS_KPXC_DB", kpxcDB.EnvVar)
	assert.Equal(t, "kpxc-db", kpxcDB.Flag)
	assert.Equal(t, "envsecrets", kpxcDB.Default)
	assert.Equal(t, "global", kpxcDB.Scope)
	assert.NotEmpty(t, kpxcDB.Usage)

	// ksm_config — global scope
	ksmConfig := fields[4]
	assert.Equal(t, "ENVSECRETS_KSM_CONFIG", ksmConfig.EnvVar)
	assert.Equal(t, "ksm-config", ksmConfig.Flag)
	assert.Equal(t, "~/.config/envsecrets/ksm-config.json", ksmConfig.Default)
	assert.Equal(t, "global", ksmConfig.Scope)
	assert.NotEmpty(t, ksmConfig.Usage)

	// ksm_folder — global scope
	ksmFolder := fields[5]
	assert.Equal(t, "ENVSECRETS_KSM_FOLDER", ksmFolder.EnvVar)
	assert.Equal(t, "ksm-folder", ksmFolder.Flag)
	assert.Equal(t, "", ksmFolder.Default)
	assert.Equal(t, "global", ksmFolder.Scope)
	assert.NotEmpty(t, ksmFolder.Usage)

	// template — gen-env scope
	template := fields[6]
	assert.Equal(t, "ENVSECRETS_TEMPLATE", template.EnvVar)
	assert.Equal(t, "template", template.Flag)
	assert.Equal(t, ".env.tpl", template.Default)
	assert.Equal(t, "gen-env", template.Scope)
	assert.NotEmpty(t, template.Usage)

	// output — gen-env scope
	output := fields[7]
	assert.Equal(t, "ENVSECRETS_OUTPUT", output.EnvVar)
	assert.Equal(t, "output", output.Flag)
	assert.Equal(t, ".env", output.Default)
	assert.Equal(t, "gen-env", output.Scope)
	assert.NotEmpty(t, output.Usage)
}

// ---------------------------------------------------------------------------
// TestValidateStem — character-set guard for vault / kpxc_db stems
// ---------------------------------------------------------------------------

func TestValidateStem(t *testing.T) {
	t.Parallel()

	valid := []string{
		"envsecrets",
		"work",
		"my-project",
		"my_project",
		"Project123",
		"a",
		"A1",
		"abc-def_ghi",
	}

	invalid := []string{
		"",
		"../escape",
		"../../etc/passwd",
		"/absolute/path",
		"has space",
		".hidden",
		"-starts-with-dash",
		"_starts_with_underscore",
		"has.dot",
		"has/slash",
		"has\\backslash",
		"has\x00null",
	}

	for _, s := range valid {
		s := s
		t.Run("valid/"+s, func(t *testing.T) {
			t.Parallel()
			assert.True(t, config.ValidateStem(s), "expected %q to be valid", s)
		})
	}

	for _, s := range invalid {
		s := s
		t.Run("invalid/"+s, func(t *testing.T) {
			t.Parallel()
			assert.False(t, config.ValidateStem(s), "expected %q to be invalid", s)
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid config passes", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Vault: "envsecrets", KpxcDB: "envsecrets"}
		assert.NoError(t, config.Validate(cfg))
	})

	t.Run("invalid vault", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Vault: "../../bad", KpxcDB: "envsecrets"}
		err := config.Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vault")
	})

	t.Run("invalid kpxc_db", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Vault: "envsecrets", KpxcDB: "/absolute/path"}
		err := config.Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "kpxc_db")
	})

	t.Run("both invalid returns joined error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Vault: "../bad", KpxcDB: "/also/bad"}
		err := config.Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vault")
		assert.Contains(t, err.Error(), "kpxc_db")
	})
}

// ---------------------------------------------------------------------------
// TestSourceFlags — bitmask type, constants, and methods
// ---------------------------------------------------------------------------

func TestSourceFlags(t *testing.T) {
	t.Parallel()

	var f config.SourceFlags

	// Zero value has no bits set.
	assert.False(t, f.Has(config.SourceFile))
	assert.False(t, f.Has(config.SourceEnv))
	assert.False(t, f.Has(config.SourceFlag))
	assert.Equal(t, "none", f.String())

	// With sets bits without mutating the original.
	f2 := f.With(config.SourceFile)
	assert.False(t, f.Has(config.SourceFile), "With must not mutate receiver")
	assert.True(t, f2.Has(config.SourceFile))
	assert.False(t, f2.Has(config.SourceEnv))
	assert.Equal(t, "file", f2.String())

	// Multiple bits.
	f3 := f2.With(config.SourceEnv).With(config.SourceFlag)
	assert.True(t, f3.Has(config.SourceFile))
	assert.True(t, f3.Has(config.SourceEnv))
	assert.True(t, f3.Has(config.SourceFlag))
	assert.Equal(t, "file|env|flag", f3.String())

	// Constants are distinct single bits.
	assert.NotEqual(t, config.SourceFile, config.SourceEnv)
	assert.NotEqual(t, config.SourceEnv, config.SourceFlag)
	assert.NotEqual(t, config.SourceFile, config.SourceFlag)
}

// ---------------------------------------------------------------------------
// TestApplyFlag — verifies flag override logic
// ---------------------------------------------------------------------------

func TestApplyFlag(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Vault:   "envsecrets",
		OpVault: "Envsecrets",
		Sources: map[string]config.SourceState{
			"vault":    {Active: config.ActiveDefault},
			"op_vault": {Active: config.ActiveFile, Flags: config.SourceFile},
		},
	}

	config.ApplyFlag(cfg, "op_vault", "MyVault")

	// Value must be updated.
	assert.Equal(t, "MyVault", cfg.OpVault)
	// Active must be "flag", SourceFlag set, and FlagValue captured.
	assert.Equal(t, config.ActiveFlag, cfg.Sources["op_vault"].Active)
	assert.True(t, cfg.Sources["op_vault"].Flags.Has(config.SourceFlag))
	assert.Equal(t, "MyVault", cfg.Sources["op_vault"].FlagValue)
	// Pre-existing SourceFile bit must be preserved.
	assert.True(t, cfg.Sources["op_vault"].Flags.Has(config.SourceFile))

	// Unrelated fields must be unaffected.
	assert.Equal(t, "envsecrets", cfg.Vault)
	assert.Equal(t, config.ActiveDefault, cfg.Sources["vault"].Active)
	assert.False(t, cfg.Sources["vault"].Flags.Has(config.SourceFlag))
}

func TestApplyFlag_NilSources(t *testing.T) {
	t.Parallel()

	// Sources is nil — ApplyFlag must initialise the map.
	cfg := &config.Config{Vault: "envsecrets"}

	config.ApplyFlag(cfg, "vault", "newvault")

	assert.Equal(t, "newvault", cfg.Vault)
	require.NotNil(t, cfg.Sources)
	assert.Equal(t, config.ActiveFlag, cfg.Sources["vault"].Active)
	assert.True(t, cfg.Sources["vault"].Flags.Has(config.SourceFlag))
	assert.Equal(t, "newvault", cfg.Sources["vault"].FlagValue)
}

// ---------------------------------------------------------------------------
// TestGenerateConfigTemplate — verifies template content is derived from tags
// ---------------------------------------------------------------------------

func TestGenerateConfigTemplate(t *testing.T) {
	t.Parallel()

	tmpl := config.GenerateConfigTemplate()

	// Must contain all four TOML keys.
	assert.Contains(t, tmpl, "vault =")
	assert.Contains(t, tmpl, "op_vault =")
	assert.Contains(t, tmpl, "template =")
	assert.Contains(t, tmpl, "output =")

	// Must contain CLI flag names.
	assert.Contains(t, tmpl, "--vault")
	assert.Contains(t, tmpl, "--op-vault")
	assert.Contains(t, tmpl, "--template")
	assert.Contains(t, tmpl, "--output")

	// Must contain environment variable names.
	assert.Contains(t, tmpl, "ENVSECRETS_VAULT")
	assert.Contains(t, tmpl, "ENVSECRETS_OP_VAULT")
	assert.Contains(t, tmpl, "ENVSECRETS_TEMPLATE")
	assert.Contains(t, tmpl, "ENVSECRETS_OUTPUT")

	// Must contain default values (quoted TOML strings).
	assert.Contains(t, tmpl, `"envsecrets"`)
	assert.Contains(t, tmpl, `"Envsecrets"`)
	assert.Contains(t, tmpl, `".env.tpl"`)
	assert.Contains(t, tmpl, `".env"`)
}
