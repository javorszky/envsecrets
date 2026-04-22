package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

// SourceFlags is a bitmask that records which sources have contributed a value
// for a single config field. Use the SourceFile / SourceEnv / SourceFlag
// constants and the Has / With methods rather than manipulating bits directly.
type SourceFlags byte

const (
	// SourceFile is set when the key is explicitly present in the config file.
	SourceFile SourceFlags = 1 << iota
	// SourceEnv is set when the corresponding environment variable is non-empty.
	SourceEnv
	// SourceFlag is set when the CLI flag was explicitly provided by the user.
	SourceFlag
)

// ActiveSource identifies which configuration source is currently active
// (highest-priority) for a single config field.
type ActiveSource uint8

const (
	ActiveDefault ActiveSource = iota // built-in default; no file/env/flag present
	ActiveFile                        // config file value is active
	ActiveEnv                         // environment variable is active
	ActiveFlag                        // CLI flag is active
)

// Has reports whether all bits in flag are set in f.
func (f SourceFlags) Has(flag SourceFlags) bool {
	return f&flag != 0
}

// With returns a copy of f with the bits in flag set.
func (f SourceFlags) With(flag SourceFlags) SourceFlags {
	return f | flag
}

// String returns a human-readable description of the set flags, e.g. "file|env".
// Returns "none" when no flags are set. Used in test failure messages.
func (f SourceFlags) String() string {
	if f == 0 {
		return "none"
	}

	var parts []string

	if f.Has(SourceFile) {
		parts = append(parts, "file")
	}

	if f.Has(SourceEnv) {
		parts = append(parts, "env")
	}

	if f.Has(SourceFlag) {
		parts = append(parts, "flag")
	}

	return strings.Join(parts, "|")
}

// SourceState records which sources have a value for a single config field
// and which source is currently active (highest priority).
type SourceState struct {
	Flags     SourceFlags  // bitmask: SourceFile | SourceEnv | SourceFlag
	Active    ActiveSource // which source wins (highest-priority non-zero source)
	FileValue string       // value as read from config file; empty when SourceFile is not set
	EnvValue  string       // value of the env var at load time; empty when SourceEnv is not set
	FlagValue string       // value passed via CLI flag; empty when SourceFlag is not set
}

// Config holds all resolved configuration for the application.
// The Sources field describes where each value came from.
// Flag overrides are NOT applied here — the cmd layer must do that after Load returns.
type Config struct {
	Vault          string `cfg:"vault"           env:"ENVSECRETS_VAULT"           flag:"vault"           default:"envsecrets" scope:"global"  usage:"local keychain file name — secrets stored at ~/.local/share/envsecrets/<vault>.keychain"`
	OpVault        string `cfg:"op_vault"        env:"ENVSECRETS_OP_VAULT"        flag:"op-vault"        default:"Envsecrets" scope:"global"  usage:"1Password vault name — created automatically on first write if it does not exist"`
	DurableBackend string `cfg:"durable_backend" env:"ENVSECRETS_DURABLE_BACKEND" flag:"durable-backend" default:"1password"  scope:"global"  usage:"durable secret backend: \"1password\" or \"keepassxc\""`
	KpxcDB         string `cfg:"kpxc_db"         env:"ENVSECRETS_KPXC_DB"         flag:"kpxc-db"         default:"envsecrets" scope:"global"  usage:"KeePassXC database stem name — stored as ~/.local/share/envsecrets/<name>.kdbx"`
	Template       string `cfg:"template"        env:"ENVSECRETS_TEMPLATE"        flag:"template"        default:".env.tpl"   scope:"gen-env" usage:"gen-env template file path — may contain secret:KEY references"`
	Output         string `cfg:"output"          env:"ENVSECRETS_OUTPUT"          flag:"output"          default:".env"       scope:"gen-env" usage:"gen-env output file path — add this file to .gitignore"`

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
	return ScopedFields("global")
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

// GetValue returns the current string value of the Config field described by m.
func GetValue(cfg *Config, m FieldMeta) string {
	return reflect.ValueOf(cfg).Elem().Field(m.FieldIndex).String()
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
	s.Flags = s.Flags.With(SourceFlag)
	s.Active = ActiveFlag
	s.FlagValue = value
	cfg.Sources[key] = s
}

// validStem is the regexp that vault and kpxc_db values must satisfy.
// A stem must start with an ASCII letter or digit and may only contain ASCII
// letters, digits, hyphens (-), and underscores (_). This prevents path
// traversal (no slashes or dots), shell meta-characters, and characters that
// are invalid in macOS keychain service names or file names.
var validStem = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidateStem reports whether s is a valid stem name for use as vault or
// kpxc_db. Valid stems match [a-zA-Z0-9][a-zA-Z0-9_-]*.
func ValidateStem(s string) bool {
	return validStem.MatchString(s)
}

// Validate checks configuration fields that have restricted character sets.
// Currently validates vault and kpxc_db, both of which are used as file-name
// stems and must not contain path separators or other special characters.
// Returns a joined error listing every invalid field.
func Validate(cfg *Config) error {
	var errs []error

	if !ValidateStem(cfg.Vault) {
		errs = append(errs, fmt.Errorf(
			"vault %q: must start with a letter or digit and contain only letters, digits, hyphens, and underscores",
			cfg.Vault,
		))
	}

	if !ValidateStem(cfg.KpxcDB) {
		errs = append(errs, fmt.Errorf(
			"kpxc_db %q: must start with a letter or digit and contain only letters, digits, hyphens, and underscores",
			cfg.KpxcDB,
		))
	}

	return errors.Join(errs...)
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
	fileValues := make(map[string]string)

	if fileFound {
		fv := viper.New()
		fv.SetConfigFile(v.ConfigFileUsed())

		if fv.ReadInConfig() == nil {
			for _, k := range fv.AllKeys() {
				fileKeys[k] = true
				fileValues[k] = fv.GetString(k)
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

		envVal := os.Getenv(m.EnvVar)

		var flags SourceFlags
		if fileKeys[m.Key] {
			flags = flags.With(SourceFile)
		}

		if envVal != "" {
			flags = flags.With(SourceEnv)
		}

		state := SourceState{
			Flags:     flags,
			FileValue: fileValues[m.Key],
			EnvValue:  envVal,
		}

		switch {
		case flags.Has(SourceEnv):
			state.Active = ActiveEnv
		case flags.Has(SourceFile):
			state.Active = ActiveFile
		default:
			state.Active = ActiveDefault
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
