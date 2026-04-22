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
	"time"
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

	return ParsePasswordOutput(out), nil
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

// ParsePasswordOutput extracts the secret value from the raw bytes returned by
// `security find-generic-password -w`. It trims exactly one trailing newline
// (the one the CLI appends to its output), while preserving any internal
// newlines and any trailing newlines that are part of the value itself (e.g.
// a secret that intentionally ends with a blank line).
func ParsePasswordOutput(out []byte) string {
	return strings.TrimSuffix(string(out), "\n")
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
//
// If the login-keychain entry holding the file's password is ever lost,
// readKeychainPassword falls back to the access-details file written to
// ~/Documents at creation time and restores the entry automatically.
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
//  6. Write access-details file to ~/Documents
//
// Subsequent use (file exists):
//  1. Read password from login keychain (falls back to ~/Documents access file)
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

	// Write a human-readable access-details file to ~/Documents so the user
	// always has a copy of the password. This is best-effort: a failure here
	// does not prevent the keychain from working.
	if err := c.writeAccessFile(password); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: could not write keychain access file: %v\n", err)
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
// login keychain under service "envsecrets-keychain-<vault>". The -U flag
// makes add-generic-password act as an upsert so a stale entry from a
// previous installation never causes a "duplicate item" failure.
func (c *Client) storeKeychainPassword(ctx context.Context, password string) error {
	svc := "envsecrets-keychain-" + c.vault

	cmd := exec.CommandContext(ctx,
		"security",
		"add-generic-password",
		"-U", // update existing entry if present
		"-a", currentUser(),
		"-s", svc,
		"-w", password,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("storing password for %q: %w\n%s", svc, err, out)
	}

	return nil
}

// readKeychainPassword retrieves the keychain file's password.
//
// Primary source: the login keychain (service "envsecrets-keychain-<vault>").
//
// Fallback: if that entry is missing (e.g. after a machine migration or a
// keychain reset), the access-details file written to ~/Documents at creation
// time is read instead.  The entry is then restored in the login keychain so
// subsequent calls are fast again.
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
	if err == nil {
		return strings.TrimRight(string(out), "\n"), nil
	}

	// Login-keychain entry missing — try the access-details file.
	password, fileErr := c.readAccessFile()
	if fileErr != nil {
		return "", fmt.Errorf(
			"reading password for %q from login keychain (%w) and from access file %s (%v)",
			svc, err, c.accessFilePath(), fileErr,
		)
	}

	// Restore the login-keychain entry so next time is seamless.
	_ = c.storeKeychainPassword(ctx, password)

	return password, nil
}

// --- access-details file -----------------------------------------------------

// accessFilePath returns the path of the access-details file created when the
// keychain vault is first set up. It lives in ~/Documents so the user can
// always find the password even if the login-keychain entry is lost.
func (c *Client) accessFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "envsecrets-"+c.vault+"-keychain-access.txt")
}

// writeAccessFile writes a plaintext file to ~/Documents containing the
// keychain password and instructions for opening it without the CLI.
// The file is 0600 (owner-read/write only).
func (c *Client) writeAccessFile(password string) error {
	path := c.accessFilePath()

	content := fmt.Sprintf(`envsecrets Keychain Access Details
===================================
Created: %s

Vault name    : %s
Keychain file : %s

KEEP THIS FILE SAFE — it contains the password to your envsecrets keychain.
Anyone who can read this file can unlock the keychain and read your secrets.

To open the keychain in Keychain Access (GUI):
  1. Open Keychain Access  (Applications > Utilities > Keychain Access)
  2. File > Add Keychain...
  3. Select the keychain file shown above
  4. Enter the password when prompted (see below)

To unlock and inspect the keychain from the terminal:
  security unlock-keychain -p '<password>' '%s'

The envsecrets CLI stores this password in your login keychain for everyday
use. If that entry is ever lost, envsecrets will read it from this file and
restore the login-keychain entry automatically — no manual steps needed.

# --- do not edit below this line ---
vault: %s
keychain-path: %s
password: %s
`,
		time.Now().Format("2006-01-02"),
		c.vault,
		c.keychainPath,
		c.keychainPath,
		c.vault,
		c.keychainPath,
		password,
	)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing access file: %w", err)
	}

	fmt.Fprintf(os.Stderr,
		"info: keychain access details written to %s\n"+
			"      Keep this file safe — it contains your keychain password.\n",
		path,
	)

	return nil
}

// readAccessFile parses the keychain password from the access-details file.
// The file must contain a line of the form "password: <hex>" in its
// machine-readable section (below the "# --- do not edit" separator).
func (c *Client) readAccessFile() (string, error) {
	data, err := os.ReadFile(c.accessFilePath())
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "password: ") {
			return strings.TrimPrefix(line, "password: "), nil
		}
	}

	return "", errors.New("password field not found in access file")
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
