package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all resolved configuration for the application.
// The Sources field describes where each value came from.
// Flag overrides are NOT applied here — the cmd layer must do that after Load returns.
type Config struct {
	// Vault is the name used for the dedicated local keychain file.
	// The file lives at ~/.local/share/envsecrets/<vault>.keychain.
	Vault string
	// OpVault is the 1Password vault name where secrets are stored.
	// It is recommended to use a dedicated vault (not "Private") to keep
	// envsecrets secrets organised and separate from personal items.
	OpVault string
	// Template is the path to the gen-env template file.
	Template string
	// Output is the path to the gen-env output file.
	Output string
	// FilePath is the resolved path to the config file (may not exist on disk).
	FilePath string
	// FileFound is true if the config file was successfully read.
	FileFound bool
	// Sources describes where each config value came from.
	Sources Sources
}

// Sources describes the origin of each config value.
// Possible values: "default", "config file", "env ($VAR_NAME)", "flag (--flag-name)".
// Flag sources are set by the cmd layer after Load returns.
type Sources struct {
	Vault    string
	OpVault  string
	Template string
	Output   string
}

// Load reads the config file (if present) and environment variables, applying
// built-in defaults for any values not set. The configFlagValue parameter is
// the value of the --config CLI flag; pass an empty string when the flag was
// not provided. $ENVSECRETS_CONFIG is checked as a fallback before the default
// path (~/.config/envsecrets.toml).
//
// Load does NOT apply CLI flag overrides for vault/template/output. The cmd
// layer is responsible for checking flag.Changed and mutating the returned
// *Config and its Sources accordingly.
func Load(configFlagValue string) *Config {
	v := viper.New()

	// Determine the config file location:
	// priority: --config flag > $ENVSECRETS_CONFIG env var > default path
	resolvedPath := resolvePath(configFlagValue)
	if configFlagValue != "" || os.Getenv("ENVSECRETS_CONFIG") != "" {
		// Exact file specified — tell viper to use it directly.
		v.SetConfigFile(resolvedPath)
	} else {
		// Default location: ~/.config/envsecrets.toml
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".config"))
		}
		v.SetConfigName("envsecrets")
		v.SetConfigType("toml")
	}

	// Environment variable bindings (second highest priority after flags).
	_ = v.BindEnv("vault", "ENVSECRETS_VAULT")
	_ = v.BindEnv("op_vault", "ENVSECRETS_OP_VAULT")
	_ = v.BindEnv("template", "ENVSECRETS_TEMPLATE")
	_ = v.BindEnv("output", "ENVSECRETS_OUTPUT")

	// Built-in defaults (lowest priority).
	v.SetDefault("vault", "envsecrets")
	v.SetDefault("op_vault", "Private")
	v.SetDefault("template", ".env.tpl")
	v.SetDefault("output", ".env")

	// Read config file. A missing file is not an error.
	fileFound := v.ReadInConfig() == nil

	// Track which keys were explicitly present in the config file.
	// A separate viper instance (no env bindings, no defaults) is used so
	// AllKeys() only returns keys literally written in the file.
	fileKeys := make(map[string]bool)
	if fileFound {
		fv := viper.New()
		fv.SetConfigFile(v.ConfigFileUsed())
		if fv.ReadInConfig() == nil {
			for _, k := range fv.AllKeys() {
				fileKeys[k] = true
			}
		}
	}

	cfg := &Config{
		Vault:     v.GetString("vault"),
		OpVault:   v.GetString("op_vault"),
		Template:  v.GetString("template"),
		Output:    v.GetString("output"),
		FileFound: fileFound,
	}

	if fileFound {
		cfg.FilePath = v.ConfigFileUsed()
	} else {
		cfg.FilePath = resolvedPath
	}

	cfg.Sources.Vault = sourceOf("vault", "ENVSECRETS_VAULT", fileKeys)
	cfg.Sources.OpVault = sourceOf("op_vault", "ENVSECRETS_OP_VAULT", fileKeys)
	cfg.Sources.Template = sourceOf("template", "ENVSECRETS_TEMPLATE", fileKeys)
	cfg.Sources.Output = sourceOf("output", "ENVSECRETS_OUTPUT", fileKeys)

	return cfg
}

// resolvePath returns the explicit config file path to use, honouring
// --config flag first, then $ENVSECRETS_CONFIG, then the default location.
// The returned path may not exist on disk.
func resolvePath(configFlagValue string) string {
	if configFlagValue != "" {
		return configFlagValue
	}
	if v := os.Getenv("ENVSECRETS_CONFIG"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.config/envsecrets.toml"
	}
	return filepath.Join(home, ".config", "envsecrets.toml")
}

// sourceOf returns "default", "config file", or "env ($VAR_NAME)" for a given
// config key. Flag overrides ("flag (--name)") are not handled here — they are
// set in the cmd layer after Load returns.
func sourceOf(key, envVar string, fileKeys map[string]bool) string {
	if os.Getenv(envVar) != "" {
		return "env ($" + envVar + ")"
	}
	if fileKeys[key] {
		return "config file"
	}
	return "default"
}
