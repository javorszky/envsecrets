// Package keeper implements the SecretStore interface backed by Keeper Secrets Manager.
// Authentication uses a One-Time Access Token (OAT) for initial device registration;
// subsequent calls use stored cryptographic credentials in the config JSON file.
// The SDK authenticates at device level — SSO does not participate after the initial
// OAT setup.
//
// # Concurrency
//
// Client is safe for concurrent use: loadManager is mutex-guarded. However,
// the underlying *ksm.SecretsManager performs synchronous network calls and
// the SDK provides no concurrency guarantees of its own. Callers must not
// invoke Get/Set/Delete/List concurrently on the same Client.
package keeper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	ksm "github.com/keeper-security/secrets-manager-go/core"
	"golang.org/x/term"
)

// ErrNotFound is returned when a Keeper record does not exist.
var ErrNotFound = errors.New("keeper: record not found")

// ErrUnavailable is returned when the KSM config file is absent (not yet initialised).
var ErrUnavailable = errors.New("keeper: not initialised — run a write command to set up credentials")

// ErrDuplicateTitles is returned when more than one record shares the same title.
// Keeper does not enforce title uniqueness, so duplicates must be resolved in the
// Keeper web console before this backend can safely read or write the affected key.
var ErrDuplicateTitles = errors.New("keeper: duplicate record titles")

// ErrWrongType is returned when a record's type is not "login".
// This backend stores secrets exclusively as login records (title = key,
// password field = value). A conflicting non-login record must be removed or
// renamed in the Keeper web console.
var ErrWrongType = errors.New("keeper: record is not a login type")

// Client implements secrets.SecretStore backed by Keeper Secrets Manager.
type Client struct {
	configPath string // path to ksm-config.json; stores device credentials after first OAT init
	folderUID  string // shared folder UID for creating new records; may be empty for read-only use

	mu sync.Mutex
	sm *ksm.SecretsManager // lazily initialised from config file; nil until first loadManager call
}

// New returns a Client. configPath supports the "~/" prefix; the expansion is
// attempted once at construction time. If os.UserHomeDir fails (very unlikely on
// macOS), the literal "~/..." path is kept and will produce a clear ENOENT later.
func New(configPath, folderUID string) *Client {
	if strings.HasPrefix(configPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			configPath = filepath.Join(home, configPath[2:])
		}
	}
	return &Client{configPath: configPath, folderUID: folderUID}
}

// validateKey rejects keys that would cause silent lookup failures.
// An empty key, one with leading/trailing whitespace, or one with embedded
// control characters creates records that are invisible or indistinct in list
// output and shell completion.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("keeper: key must not be empty")
	}
	if strings.TrimSpace(key) != key {
		return fmt.Errorf("keeper: key %q must not have leading or trailing whitespace", key)
	}
	if strings.ContainsAny(key, "\n\r\t\x00") {
		return fmt.Errorf("keeper: key %q must not contain control characters", key)
	}
	return nil
}

// Available returns true when the config file exists on disk. It does not make a
// network call; EnsureVault verifies that the credentials are actually usable.
func (c *Client) Available(_ context.Context) bool {
	_, err := os.Stat(c.configPath)
	return err == nil
}

// EnsureVault initialises KSM device credentials when the config file does not
// yet exist, or verifies them when it does.
//
// First-time path: prompts for a One-Time Access Token without echo (TTY) or
// reads it from a pipe (non-TTY), validates the token format before handing it
// to the SDK (prevents the SDK from logging the raw token on malformed input),
// registers the device with KSM, and secures the written config file to 0600.
// The file is pre-created at 0600 before the SDK is initialised, eliminating
// the race window where the SDK's os.Create would write at 0666. On probe
// failure the partial config is removed so the next run re-prompts rather than
// failing with a stale file.
//
// Subsequent path: loads the config and runs a GetSecrets probe to confirm
// the stored credentials are still valid.
//
// Returns (true, nil) when the config was newly created; (false, nil) when it
// already existed and credentials verified successfully.
func (c *Client) EnsureVault(_ context.Context) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0700); err != nil {
		return false, fmt.Errorf("keeper: create config dir: %w", err)
	}

	if _, err := os.Stat(c.configPath); errors.Is(err, os.ErrNotExist) {
		oat, err := readOAT()
		if err != nil {
			return false, err
		}

		// Pre-create the config file at 0600 so the SDK never gets the chance
		// to call os.Create (which writes at 0666). The SDK's createConfigFileIfMissing
		// checks existence before creating, so it will find this file and skip its own create.
		f, createErr := os.OpenFile(c.configPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if createErr != nil && !errors.Is(createErr, os.ErrExist) {
			return false, fmt.Errorf("keeper: create config file: %w", createErr)
		}
		if createErr == nil {
			_, _ = f.WriteString("{}")
			_ = f.Close()
		}

		sm, err := c.initManager(oat)
		if err != nil {
			_ = os.Remove(c.configPath)
			return false, err
		}

		// Probe confirms the OAT was accepted and the private key is now stored.
		if _, probeErr := sm.GetSecrets([]string{}); probeErr != nil {
			// Remove the partial config so the next run re-prompts rather than
			// failing with a stale/corrupt file.
			_ = os.Remove(c.configPath)
			return false, fmt.Errorf("keeper: verify OAT: %w", probeErr)
		}

		// Don't cache the OAT-initialised manager — loadManager will build a
		// clean instance from the now-populated config file on the next call.

		return true, nil
	}

	// Config exists — verify credentials.
	sm, err := c.loadManager()
	if err != nil {
		return false, err
	}

	if _, verifyErr := sm.GetSecrets([]string{}); verifyErr != nil {
		return false, fmt.Errorf("keeper: verify credentials: %w", verifyErr)
	}

	return false, nil
}

// Get retrieves the password field of the login record whose title equals key.
// Returns ErrNotFound when no matching record exists, ErrDuplicateTitles when
// the title matches more than one record, and ErrWrongType when the matching
// record is not a login record.
func (c *Client) Get(_ context.Context, key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}

	sm, err := c.loadManager()
	if err != nil {
		return "", err
	}

	record, err := c.lookupOne(sm, key)
	if err != nil {
		return "", err
	}

	if record == nil {
		return "", fmt.Errorf("keeper: get %q: %w", key, ErrNotFound)
	}

	return record.Password(), nil
}

// Set creates or updates the login record whose title equals key.
// Update-first: if the record already exists and is a login record, its
// password field is updated. If absent, a new login record is created in
// c.folderUID (returns a clear error if folderUID is empty).
// Returns ErrDuplicateTitles or ErrWrongType when the key cannot be written
// safely.
func (c *Client) Set(_ context.Context, key, value string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	sm, err := c.loadManager()
	if err != nil {
		return err
	}

	record, err := c.lookupOne(sm, key)
	if err != nil {
		return err
	}

	if record != nil {
		record.SetPassword(value)
		if err := sm.Save(record); err != nil {
			return fmt.Errorf("keeper: update %q: %w", key, err)
		}

		return nil
	}

	if c.folderUID == "" {
		return fmt.Errorf("keeper: cannot create secret %q: ksm-folder is not configured (set ENVSECRETS_KSM_FOLDER or --ksm-folder)", key)
	}

	rc := ksm.NewRecordCreate("login", key)
	rc.Fields = append(rc.Fields, ksm.NewLogin("envsecrets"))
	rc.Fields = append(rc.Fields, ksm.NewPassword(value))

	if _, err := sm.CreateSecretWithRecordData("", c.folderUID, rc); err != nil {
		return fmt.Errorf("keeper: create %q: %w", key, err)
	}

	return nil
}

// Delete removes the login record whose title equals key.
// Returns ErrNotFound when the record is absent (caller may treat this as a
// no-op), ErrDuplicateTitles or ErrWrongType when the key cannot be resolved
// safely.
func (c *Client) Delete(_ context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	sm, err := c.loadManager()
	if err != nil {
		return err
	}

	record, err := c.lookupOne(sm, key)
	if err != nil {
		return err
	}

	if record == nil {
		return fmt.Errorf("keeper: delete %q: %w", key, ErrNotFound)
	}

	if _, err := sm.DeleteSecrets([]string{record.Uid}); err != nil {
		return fmt.Errorf("keeper: delete %q: %w", key, err)
	}

	return nil
}

// List returns the titles of all login records accessible to this KSM application.
// Non-login records are silently skipped. Records whose titles are shared by
// multiple entries are warned about and excluded — duplicates must be resolved
// in the Keeper web console before they can be used.
//
// Note: each call makes one full-vault network fetch. For bulk operations such
// as Sync this is efficient (one fetch total); for N sequential Gets after
// List it degrades to N+1 fetches because GetSecretsByTitle re-fetches
// internally.
func (c *Client) List(_ context.Context) ([]string, error) {
	sm, err := c.loadManager()
	if err != nil {
		return nil, err
	}

	records, err := sm.GetSecrets([]string{})
	if err != nil {
		return nil, fmt.Errorf("keeper: list: %w", err)
	}

	// Count how many login records share each title.
	counts := make(map[string]int, len(records))
	for _, r := range records {
		if r.Type() == "login" {
			counts[r.Title()]++
		}
	}

	keys := make([]string, 0, len(counts))
	for title, n := range counts {
		if n > 1 {
			fmt.Fprintf(os.Stderr, "warning: keeper: skipping %q — %d records share this title; resolve duplicates in the Keeper web console\n", title, n)
			continue
		}
		keys = append(keys, title)
	}

	sort.Strings(keys)
	return keys, nil
}

// lookupOne resolves key to exactly one login record or returns (nil, nil) when
// absent. Errors are returned for transient failures, duplicate titles, and
// non-login record types — all of which the caller must not silently swallow.
func (c *Client) lookupOne(sm *ksm.SecretsManager, key string) (*ksm.Record, error) {
	records, err := sm.GetSecretsByTitle(key)
	if err != nil {
		return nil, fmt.Errorf("keeper: lookup %q: %w", key, err)
	}

	switch len(records) {
	case 0:
		return nil, nil
	case 1:
		r := records[0]
		if r.Type() != "login" {
			return nil, fmt.Errorf(
				"keeper: key %q is stored as a %q record, not \"login\" — "+
					"this backend only manages login records: %w",
				key, r.Type(), ErrWrongType,
			)
		}
		return r, nil
	default:
		return nil, fmt.Errorf(
			"keeper: key %q matches %d records — titles must be unique; "+
				"resolve duplicates in the Keeper web console: %w",
			key, len(records), ErrDuplicateTitles,
		)
	}
}

// loadManager returns the cached SecretsManager, initialising it from the config
// file on the first call. The manager is reused across Get/Set/Delete/List calls
// within the same process to avoid repeated config-file reads.
func (c *Client) loadManager() (*ksm.SecretsManager, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sm != nil {
		return c.sm, nil
	}

	sm := ksm.NewSecretsManager(&ksm.ClientOptions{
		Config: newSecureStorage(c.configPath),
	})
	if sm == nil {
		return nil, fmt.Errorf("keeper: failed to load config from %s: %w", c.configPath, ErrUnavailable)
	}

	c.sm = sm
	return sm, nil
}

// initManager creates a SecretsManager using a One-Time Access Token for
// first-time device registration. It must only be called during EnsureVault's
// first-time setup path; the resulting manager is NOT cached because it was
// constructed with the OAT, which is one-time-use. Use loadManager for all
// subsequent calls.
func (c *Client) initManager(oat string) (*ksm.SecretsManager, error) {
	sm := ksm.NewSecretsManager(&ksm.ClientOptions{
		Token:  oat,
		Config: newSecureStorage(c.configPath),
	})
	if sm == nil {
		return nil, fmt.Errorf("keeper: failed to initialise SDK with provided token")
	}

	return sm, nil
}

// readOAT prompts for and returns the One-Time Access Token. When stdin is a
// TTY, input is hidden (no echo). When stdin is a pipe, the token is read
// from the pipe directly (it is never visible on screen anyway).
// The token is validated before being returned so that a malformed input
// is rejected here rather than leaking to the SDK's logger.
func readOAT() (string, error) {
	fmt.Fprint(os.Stderr, "Enter your One-Time Access Token (from KSM console > Applications): ")

	var oat string

	if term.IsTerminal(int(os.Stdin.Fd())) {
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // ReadPassword suppresses the trailing newline.
		if err != nil {
			return "", fmt.Errorf("keeper: read OAT: %w", err)
		}
		oat = string(raw)
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			oat = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("keeper: read OAT: %w", err)
		}
	}

	oat = strings.TrimSpace(oat)
	if oat == "" {
		return "", fmt.Errorf("keeper: One-Time Access Token must not be empty")
	}

	if err := validateOAT(oat); err != nil {
		return "", err
	}

	return oat, nil
}

// validateOAT checks the token format before it reaches the SDK.
// The SDK logs the raw token via klog.Warning when the format is malformed
// (more than one colon, or empty host/key parts). We reject those cases here
// so the token never appears in logs.
//
// Valid formats accepted by the SDK:
//   - "BASE64KEY"            — legacy format, no region prefix
//   - "REGION:BASE64KEY"     — e.g. "US:BASE64KEY" (recommended)
//   - "hostname:BASE64KEY"   — e.g. "keepersecurity.com:BASE64KEY"
func validateOAT(oat string) error {
	if !strings.Contains(oat, ":") {
		// Legacy format — no region prefix. SDK accepts this without logging.
		return nil
	}

	parts := strings.Split(oat, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("keeper: invalid One-Time Access Token format: expected HOST:BASE64KEY (e.g. US:xxxxxxx), got %d colon-separated parts", len(parts))
	}

	return nil
}

// secureStorage wraps the SDK's IKeyValueStorage and chmods the config file to
// 0600 after every write. The SDK's fileKeyValueStorage writes at 0666; without
// this wrapper the private key is world-readable between the SDK write and any
// deferred chmod call.
type secureStorage struct {
	inner      ksm.IKeyValueStorage
	configPath string
}

func newSecureStorage(configPath string) *secureStorage {
	return &secureStorage{
		inner:      ksm.NewFileKeyValueStorage(configPath),
		configPath: configPath,
	}
}

func (s *secureStorage) lockDown() {
	// Best-effort: chmod failures are non-fatal. The data is written and the
	// permissions will be corrected on the next successful lockDown call.
	_ = os.Chmod(filepath.Dir(s.configPath), 0700)
	_ = os.Chmod(s.configPath, 0600)
}

func (s *secureStorage) ReadStorage() map[string]interface{} { return s.inner.ReadStorage() }
func (s *secureStorage) Contains(key ksm.ConfigKey) bool     { return s.inner.Contains(key) }
func (s *secureStorage) IsEmpty() bool                       { return s.inner.IsEmpty() }
func (s *secureStorage) Get(key ksm.ConfigKey) string        { return s.inner.Get(key) }

func (s *secureStorage) SaveStorage(updatedConfig map[string]interface{}) {
	s.inner.SaveStorage(updatedConfig)
	s.lockDown()
}

func (s *secureStorage) Set(key ksm.ConfigKey, value interface{}) map[string]interface{} {
	// inner.Set calls inner.SaveStorage (not our wrapper's SaveStorage), so we
	// must chmod here explicitly rather than relying on our SaveStorage override.
	result := s.inner.Set(key, value)
	s.lockDown()
	return result
}

func (s *secureStorage) Delete(key ksm.ConfigKey) map[string]interface{} {
	result := s.inner.Delete(key)
	s.lockDown()
	return result
}

func (s *secureStorage) DeleteAll() map[string]interface{} {
	result := s.inner.DeleteAll()
	s.lockDown()
	return result
}
