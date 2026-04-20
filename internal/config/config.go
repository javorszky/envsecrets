package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

// FieldMeta holds metadata parsed from a Config struct field's struct tags.
type FieldMeta struct {
	FieldIndex int    // position in Config struct (for reflection)
	FieldName  string // Go field name, e.g. "Vault"
	Key        string // TOML / viper key, e.g. "vault"
	EnvVar     string // environment variable name, e.g. "ENVSECRETS_VAULT"
	Flag       string // cobra flag name, e.g. "op-vault"
	Default    string // built-in default value
	Usage      string // flag help string and config file comment
	Scope      string // "global" → PersistentFlags on root; "gen-env" → Flags on genEnvCmd
}

// SourceState records which sources have a value for a single config field
// and which source is currently active (highest priority).
type SourceState struct {
	FileSet bool   // key was present in the config file
	EnvSet  bool   // env var is set in the current process environment
	FlagSet bool   // cobra flag was explicitly provided on the command line
	Active  string // "default" | "file" | "env" | "flag"
}

// Config holds all resolved configuration for the application.
// The Sources field describes where each value came from.
// Flag overrides are NOT applied here — the cmd layer must do that after Load returns.
type Config struct {
	Vault    string `cfg:"vault"    env:"ENVSECRETS_VAULT"    flag:"vault"    default:"envsecrets" scope:"global"  usage:"local keychain file name — secrets stored at ~/.local/share/envsecrets/<vault>.keychain"`
	OpVault  string `cfg:"op_vault" env:"ENVSECRETS_OP_VAULT" flag:"op-vault" default:"Envsecrets" scope:"global"  usage:"1Password vault name — created automatically on first write if it does not exist"`
	Template string `cfg:"template" env:"ENVSECRETS_TEMPLATE" flag:"template" default:".env.tpl"   scope:"gen-env" usage:"gen-env template file path — may contain secret:KEY references"`
	Output   string `cfg:"output"   env:"ENVSECRETS_OUTPUT"   flag:"output"   default:".env"       scope:"gen-env" usage:"gen-env output file path — add this file to .gitignore"`

	// Non-configurable metadata — no struct tags, not iterated by AllFields.
	FilePath  string
	FileFound bool
	Sources   map[string]SourceState // keyed by cfg tag value, e.g. "vault"
}

// AllFields returns FieldMeta for every struct field that has a "cfg" tag.
// Fields without a cfg tag (FilePath, FileFound, Sources) are skipped.
func AllFields() []FieldMeta {
	t := reflect.TypeOf(Config{})
	var out []FieldMeta

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := f.Tag.Get("cfg")

		if key == "" {
			continue
		}

		out = append(out, FieldMeta{
			FieldIndex: i,
			FieldName:  f.Name,
			Key:        key,
			EnvVar:     f.Tag.Get("env"),
			Flag:       f.Tag.Get("flag"),
			Default:    f.Tag.Get("default"),
			Usage:      f.Tag.Get("usage"),
			Scope:      f.Tag.Get("scope"),
		})
	}

	return out
}

// GlobalFields returns the subset of AllFields where Scope == "global".
func GlobalFields() []FieldMeta {
	var out []FieldMeta

	for _, m := range AllFields() {
		if m.Scope == "global" {
			out = append(out, m)
		}
	}

	return out
}

// ScopedFields returns the subset of AllFields where Scope == scope.
func ScopedFields(scope string) []FieldMeta {
	var out []FieldMeta

	for _, m := range AllFields() {
		if m.Scope == scope {
			out = append(out, m)
		}
	}

	return out
}

// GetValue returns the current string value of the Config field identified by key.
func GetValue(cfg *Config, key string) string {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("cfg") == key {
			return v.Field(i).String()
		}
	}

	return ""
}

// ApplyFlag sets the Config field identified by key to value, marks FlagSet=true,
// and sets Active="flag" on the corresponding SourceState entry.
// Called by the cmd layer after cobra flag parsing when a flag was explicitly set.
func ApplyFlag(cfg *Config, key, value string) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("cfg") == key {
			v.Field(i).SetString(value)
			break
		}
	}

	if cfg.Sources == nil {
		cfg.Sources = make(map[string]SourceState)
	}

	s := cfg.Sources[key]
	s.FlagSet = true
	s.Active = "flag"
	cfg.Sources[key] = s
}

// Load reads the config file (if present) and environment variables, applying
// built-in defaults for any values not set. The configFlagValue parameter is
// the value of the --config CLI flag; pass an empty string when the flag was
// not provided. $ENVSECRETS_CONFIG is checked as a fallback before the default
// path (~/.config/envsecrets.toml).
//
// Load does NOT apply CLI flag overrides for vault/template/output. The cmd
// layer is responsible for checking flag.Changed and calling ApplyFlag.
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

	// Bind env vars and set defaults dynamically from struct tags.
	for _, m := range AllFields() {
		_ = v.BindEnv(m.Key, m.EnvVar)
		v.SetDefault(m.Key, m.Default)
	}

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
		FileFound: fileFound,
		Sources:   make(map[string]SourceState),
	}

	if fileFound {
		cfg.FilePath = v.ConfigFileUsed()
	} else {
		cfg.FilePath = resolvedPath
	}

	// Populate Config fields and SourceState dynamically.
	rv := reflect.ValueOf(cfg).Elem()

	for _, m := range AllFields() {
		rv.Field(m.FieldIndex).SetString(v.GetString(m.Key))

		state := SourceState{
			FileSet: fileKeys[m.Key],
			EnvSet:  os.Getenv(m.EnvVar) != "",
		}

		switch {
		case state.EnvSet:
			state.Active = "env"
		case state.FileSet:
			state.Active = "file"
		default:
			state.Active = "default"
		}

		cfg.Sources[m.Key] = state
	}

	return cfg
}

// GenerateConfigTemplate returns the content for a default envsecrets.toml config
// file, generated dynamically from the Config struct tags. Used by config init.
func GenerateConfigTemplate() string {
	var sb strings.Builder

	sb.WriteString("# envsecrets configuration file\n")
	sb.WriteString("# Generated by: envsecrets config init\n")
	sb.WriteString("#\n")
	sb.WriteString("# Precedence (highest to lowest):\n")
	sb.WriteString("#   1. Command-line flags    (e.g. --op-vault)\n")
	sb.WriteString("#   2. Environment variables (e.g. ENVSECRETS_OP_VAULT)\n")
	sb.WriteString("#   3. This config file\n")
	sb.WriteString("#   4. Built-in defaults\n")

	for _, m := range AllFields() {
		sb.WriteString("\n")
		sb.WriteString("# " + m.Usage + "\n")
		sb.WriteString("# CLI flag:        --" + m.Flag + "\n")
		sb.WriteString("# Environment var: " + m.EnvVar + "\n")
		sb.WriteString(m.Key + " = " + fmt.Sprintf("%q", m.Default) + "\n")
	}

	return sb.String()
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
