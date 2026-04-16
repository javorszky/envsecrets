// Package keychain wraps the macOS `security` CLI to store and retrieve
// generic passwords from a dedicated, per-vault keychain file.
//
// Each vault gets its own keychain at ~/.local/share/envsecrets/<vault>.keychain.
// The keychain file's password is generated on first use and stored in the
// user's login keychain so subsequent access is transparent.
//
// All entries are stored with:
//   - account:  current $USER
//   - service:  the key name as-is (callers should namespace, e.g. "myapp_DB_PASSWORD")
//
// Use New(vault) to create a Client.
package keychain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotFound is returned when a keychain entry does not exist.
var ErrNotFound = errors.New("keychain: entry not found")

// svcePattern matches `"svce"<blob>="<value>"` lines in dump-keychain output.
var svcePattern = regexp.MustCompile(`"svce"<blob>="([^"]*)"`)

// Client is a handle for macOS Keychain operations backed by a dedicated
// keychain file at ~/.local/share/envsecrets/<vault>.keychain.
type Client struct {
	vault        string
	keychainPath string
}

// New returns a Client targeting a per-vault keychain file.
func New(vault string) *Client {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".local", "share", "envsecrets")

	return &Client{
		vault:        vault,
		keychainPath: filepath.Join(dir, vault+".keychain"),
	}
}

// Available reports whether the macOS `security` binary is present.
func (c *Client) Available(_ context.Context) bool {
	_, err := exec.LookPath("security")
	return err == nil
}

// Get retrieves the secret for the given service key.
// Returns ErrNotFound if the entry does not exist.
func (c *Client) Get(ctx context.Context, service string) (string, error) {
	if err := c.ensure(ctx); err != nil {
		return "", fmt.Errorf("keychain ensure: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"security",
		"find-generic-password",
		"-a", currentUser(),
		"-s", service,
		"-w",
		c.keychainPath,
	)

	out, err := cmd.Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			// exit code 44 = "The specified item could not be found in the keychain."
			return "", ErrNotFound
		}

		return "", fmt.Errorf("keychain get %q: %w", service, err)
	}

	return strings.TrimRight(string(out), "\n"), nil
}

// Set stores or overwrites a secret. It deletes any existing entry first to
// avoid the "duplicate item" error that `add-generic-password` produces.
func (c *Client) Set(ctx context.Context, service, value string) error {
	if err := c.ensure(ctx); err != nil {
		return fmt.Errorf("keychain ensure: %w", err)
	}

	// Best-effort delete; ignore errors (entry may not exist yet).
	_ = c.remove(ctx, service)

	cmd := exec.CommandContext(ctx,
		"security",
		"add-generic-password",
		"-a", currentUser(),
		"-s", service,
		"-w", value,
		c.keychainPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain set %q: %w\n%s", service, err, out)
	}

	return nil
}

// Delete removes the keychain entry for the given service key.
// Returns ErrNotFound if the entry does not exist.
func (c *Client) Delete(ctx context.Context, service string) error {
	if err := c.ensure(ctx); err != nil {
		return fmt.Errorf("keychain ensure: %w", err)
	}

	return c.remove(ctx, service)
}

// List returns all service names stored in the dedicated keychain file.
func (c *Client) List(ctx context.Context) ([]string, error) {
	if err := c.ensure(ctx); err != nil {
		return nil, fmt.Errorf("keychain ensure: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"security",
		"dump-keychain",
		c.keychainPath,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("keychain list: %w", err)
	}

	return ParseDumpServices(string(out)), nil
}

// ParseDumpServices extracts unique service names from the output of
// `security dump-keychain`. Each matching `"svce"<blob>="<name>"` line
// contributes one entry. Duplicates are removed.
func ParseDumpServices(output string) []string {
	matches := svcePattern.FindAllStringSubmatch(output, -1)

	seen := make(map[string]struct{}, len(matches))
	var services []string

	for _, m := range matches {
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		services = append(services, name)
	}

	return services
}

// EnsureVault creates the dedicated keychain file if it does not already exist,
// or unlocks it if it does. Returns (true, nil) when the file was newly
// created, (false, nil) when it already existed, or (false, err) on failure.
func (c *Client) EnsureVault(ctx context.Context) (bool, error) {
	if _, err := os.Stat(c.keychainPath); errors.Is(err, os.ErrNotExist) {
		if createErr := c.createKeychainFile(ctx); createErr != nil {
			return false, createErr
		}

		return true, nil
	}

	if err := c.unlockKeychainFile(ctx); err != nil {
		return false, err
	}

	return false, nil
}

// --- keychain file lifecycle -------------------------------------------------

// ensure creates or unlocks the dedicated keychain file. Called at the start
// of every public method that accesses the file.
//
// First use (file doesn't exist):
//  1. Generate a random password
//  2. mkdir -p the parent directory
//  3. security create-keychain -p <pw> <path>
//  4. security set-keychain-settings <path>   (no auto-lock)
//  5. Store password in login keychain
//
// Subsequent use (file exists):
//  1. Read password from login keychain
//  2. security unlock-keychain -p <pw> <path>
func (c *Client) ensure(ctx context.Context) error {
	if _, err := os.Stat(c.keychainPath); errors.Is(err, os.ErrNotExist) {
		return c.createKeychainFile(ctx)
	}

	return c.unlockKeychainFile(ctx)
}

func (c *Client) createKeychainFile(ctx context.Context) error {
	password, err := generatePassword()
	if err != nil {
		return fmt.Errorf("generating keychain password: %w", err)
	}

	dir := filepath.Dir(c.keychainPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating keychain directory: %w", err)
	}

	// Create the keychain file.
	cmd := exec.CommandContext(ctx,
		"security",
		"create-keychain",
		"-p", password,
		c.keychainPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating keychain: %w\n%s", err, out)
	}

	// Disable auto-lock so the keychain stays available.
	cmd = exec.CommandContext(ctx,
		"security",
		"set-keychain-settings",
		c.keychainPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("setting keychain options: %w\n%s", err, out)
	}

	// Store the password in the login keychain for later retrieval.
	if err := c.storeKeychainPassword(ctx, password); err != nil {
		return fmt.Errorf("storing keychain password: %w", err)
	}

	return nil
}

func (c *Client) unlockKeychainFile(ctx context.Context) error {
	password, err := c.readKeychainPassword(ctx)
	if err != nil {
		return fmt.Errorf("reading keychain password: %w", err)
	}

	cmd := exec.CommandContext(ctx,
		"security",
		"unlock-keychain",
		"-p", password,
		c.keychainPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unlocking keychain: %w\n%s", err, out)
	}

	return nil
}

// storeKeychainPassword saves the keychain file's password into the user's
// login keychain under service "envsecrets-keychain-<vault>".
func (c *Client) storeKeychainPassword(ctx context.Context, password string) error {
	svc := "envsecrets-keychain-" + c.vault

	cmd := exec.CommandContext(ctx,
		"security",
		"add-generic-password",
		"-a", currentUser(),
		"-s", svc,
		"-w", password,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("storing password for %q: %w\n%s", svc, err, out)
	}

	return nil
}

// readKeychainPassword retrieves the keychain file's password from the login
// keychain.
func (c *Client) readKeychainPassword(ctx context.Context) (string, error) {
	svc := "envsecrets-keychain-" + c.vault

	cmd := exec.CommandContext(ctx,
		"security",
		"find-generic-password",
		"-a", currentUser(),
		"-s", svc,
		"-w",
	)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading password for %q: %w", svc, err)
	}

	return strings.TrimRight(string(out), "\n"), nil
}

// --- private helpers ---------------------------------------------------------

// remove is the internal implementation for deleting a keychain entry.
func (c *Client) remove(ctx context.Context, service string) error {
	cmd := exec.CommandContext(ctx,
		"security",
		"delete-generic-password",
		"-a", currentUser(),
		"-s", service,
		c.keychainPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return ErrNotFound
		}

		return fmt.Errorf("keychain delete %q: %w\n%s", service, err, out)
	}

	return nil
}

// generatePassword returns a 64-character hex string from 32 random bytes.
func generatePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// currentUser returns the OS username, falling back to LOGNAME.
func currentUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}

	return u
}
