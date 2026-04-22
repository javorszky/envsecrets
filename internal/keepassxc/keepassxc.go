// Package keepassxc wraps the keepassxc-cli tool to store and retrieve secrets.
//
// All secrets are stored as entries in a KeePass database file.
// The entry title is the key name; the secret is stored in the Password field.
//
// The database password is stored in the macOS login keychain under service
// "envsecrets-keepassxc-<vault>", mirroring the keychain backend's approach.
package keepassxc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound is returned when a KeePassXC entry does not exist.
var ErrNotFound = errors.New("keepassxc: entry not found")

// ErrUnavailable is returned when keepassxc-cli is not installed.
var ErrUnavailable = errors.New("keepassxc: keepassxc-cli unavailable")

// Client holds configuration for KeePassXC operations.
type Client struct {
	vault  string // envsecrets vault name (used for the keychain service key)
	dbPath string // absolute path to the .kdbx file, always under ~/.local/share/envsecrets/
}

// DefaultDBPath returns the KeePassXC database path for the given stem name:
// ~/.local/share/envsecrets/<stem>.kdbx
func DefaultDBPath(stem string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "envsecrets", stem+".kdbx")
}

// New returns a Client for the given vault and database stem name.
// The stem is a bare name (e.g. "envsecrets") — the database is always
// stored at ~/.local/share/envsecrets/<stem>.kdbx.
func New(vault, stem string) *Client {
	return &Client{vault: vault, dbPath: DefaultDBPath(stem)}
}

// Available reports whether keepassxc-cli is present in PATH.
func (c *Client) Available(_ context.Context) bool {
	_, err := exec.LookPath("keepassxc-cli")
	return err == nil
}

// EnsureVault creates the KeePassXC database if it does not exist, or verifies
// the stored password is readable if it does. Returns (true, nil) when newly
// created, (false, nil) when it already existed, or (false, err) on failure.
func (c *Client) EnsureVault(ctx context.Context) (bool, error) {
	_, err := os.Stat(c.dbPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("keepassxc: checking database path %q: %w", c.dbPath, err)
		}

		// File does not exist — create it.
		if err := c.createDB(ctx); err != nil {
			return false, err
		}

		return true, nil
	}

	// DB already exists — verify the stored password actually unlocks it.
	// Reading the password from the keychain alone is insufficient: the DB
	// may have been re-keyed externally or may be corrupted. Run a cheap
	// keepassxc-cli ls to confirm the file can be opened.
	pw, err := c.readPassword(ctx)
	if err != nil {
		return false, fmt.Errorf("keepassxc: cannot read database password: %w", err)
	}

	probe := exec.CommandContext(ctx,
		"keepassxc-cli", "ls",
		"--quiet",
		c.dbPath,
	)
	probe.Stdin = strings.NewReader(pw + "\n")

	if out, err := probe.CombinedOutput(); err != nil {
		return false, fmt.Errorf("keepassxc: cannot open database %q (wrong password or corrupted file): %w\n%s", c.dbPath, err, out)
	}

	return false, nil
}

// Get retrieves the Password field of the entry whose title matches key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	pw, err := c.readPassword(ctx)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "show",
		"--quiet",
		"--show-protected",
		"--attributes", "Password",
		c.dbPath,
		key,
	)
	cmd.Stdin = strings.NewReader(pw + "\n")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", classifyError(key, exitErr.Stderr)
		}

		return "", unavailableOrWrap(fmt.Sprintf("get %q", key), err)
	}

	return decodeValue(strings.TrimRight(string(out), "\n")), nil
}

// Set stores or updates the entry. It attempts an edit first; if the entry does
// not exist it creates a new entry.
//
// Arbitrary values — including multiline strings such as YAML blocks — are
// supported. Newline and carriage-return characters are percent-encoded before
// being written to the database and decoded transparently on Get, so the value
// round-trips exactly.
func (c *Client) Set(ctx context.Context, key, value string) error {
	err := c.edit(ctx, key, value)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrNotFound) {
		return err
	}

	return c.add(ctx, key, value)
}

// Delete removes the entry whose title matches key.
func (c *Client) Delete(ctx context.Context, key string) error {
	pw, err := c.readPassword(ctx)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "rm",
		"--quiet",
		c.dbPath,
		key,
	)
	cmd.Stdin = strings.NewReader(pw + "\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrUnavailable
		}

		return classifyError(key, out)
	}

	return nil
}

// List returns the titles of all root-level entries in the database,
// excluding group names and indented sub-entries.
func (c *Client) List(ctx context.Context) ([]string, error) {
	pw, err := c.readPassword(ctx)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "ls",
		"--quiet",
		c.dbPath,
	)
	cmd.Stdin = strings.NewReader(pw + "\n")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("keepassxc list: %w\n%s", err, exitErr.Stderr)
		}

		return nil, unavailableOrWrap("list", err)
	}

	return ParseListOutput(string(out)), nil
}

// ParseListOutput extracts root-level entry names from `keepassxc-cli ls` output.
// It skips group names (lines ending with "/") and indented sub-entries.
func ParseListOutput(output string) []string {
	var result []string

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // sub-entry indented under a group
		}

		if strings.HasSuffix(line, "/") {
			continue // group name
		}

		result = append(result, strings.TrimSpace(line))
	}

	return result
}

// --- private helpers -----------------------------------------------------------

func (c *Client) add(ctx context.Context, key, value string) error {
	pw, err := c.readPassword(ctx)
	if err != nil {
		return err
	}

	// keepassxc-cli add --password-prompt reads: db-password, entry-password, entry-password (confirm)
	// Newlines in the value are percent-encoded so they don't break the stdin
	// prompt protocol. Get decodes them on the way back out.
	encoded := encodeValue(value)
	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "add",
		"--quiet",
		"--password-prompt",
		c.dbPath,
		key,
	)
	cmd.Stdin = strings.NewReader(pw + "\n" + encoded + "\n" + encoded + "\n")

	if out, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrUnavailable
		}

		return fmt.Errorf("keepassxc add %q: %w\n%s", key, err, out)
	}

	return nil
}

func (c *Client) edit(ctx context.Context, key, value string) error {
	pw, err := c.readPassword(ctx)
	if err != nil {
		return err
	}

	// keepassxc-cli edit --password-prompt reads: db-password, new-entry-password, new-entry-password (confirm)
	// Newlines in the value are percent-encoded so they don't break the stdin
	// prompt protocol. Get decodes them on the way back out.
	encoded := encodeValue(value)
	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "edit",
		"--quiet",
		"--password-prompt",
		c.dbPath,
		key,
	)
	cmd.Stdin = strings.NewReader(pw + "\n" + encoded + "\n" + encoded + "\n")

	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrUnavailable
		}

		return classifyError(key, out)
	}

	return nil
}

func (c *Client) createDB(ctx context.Context) error {
	pw, err := generatePassword()
	if err != nil {
		return fmt.Errorf("generating database password: %w", err)
	}

	dir := filepath.Dir(c.dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	// db-create --set-password reads: password, password (confirm)
	cmd := exec.CommandContext(ctx,
		"keepassxc-cli", "db-create",
		"--set-password",
		c.dbPath,
	)
	cmd.Stdin = strings.NewReader(pw + "\n" + pw + "\n")

	if out, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("creating keepassxc database: %w", ErrUnavailable)
		}

		return fmt.Errorf("creating keepassxc database: %w\n%s", err, out)
	}

	if err := c.storePassword(ctx, pw); err != nil {
		// Password not persisted — the database is unrecoverable. Delete it so
		// EnsureVault can attempt a clean creation next time.
		if removeErr := os.Remove(c.dbPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("storing database password: %w (also failed to remove newly created database %q: %v)", err, c.dbPath, removeErr)
		}

		return fmt.Errorf("storing database password: %w", err)
	}

	// Best-effort: write an access-details file to ~/Documents.
	if err := c.writeAccessFile(pw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write keepassxc access file: %v\n", err)
	}

	return nil
}

// readPassword retrieves the database password from the macOS login keychain
// under service "envsecrets-keepassxc-<vault>". Falls back to the access-details
// file in ~/Documents and restores the keychain entry if the primary source is missing.
func (c *Client) readPassword(ctx context.Context) (string, error) {
	svc := "envsecrets-keepassxc-" + c.vault

	cmd := exec.CommandContext(ctx,
		"security", "find-generic-password",
		"-a", currentUser(),
		"-s", svc,
		"-w",
	)

	out, err := cmd.Output()
	if err == nil {
		return strings.TrimRight(string(out), "\n"), nil
	}

	// Login-keychain entry missing — fall back to the access-details file.
	pw, fileErr := c.readAccessFile()
	if fileErr != nil {
		return "", fmt.Errorf(
			"reading password for %q from login keychain (%w) and from access file (%v)",
			svc, err, fileErr,
		)
	}

	// Restore the login-keychain entry so next time is seamless.
	_ = c.storePassword(ctx, pw)

	return pw, nil
}

// storePassword saves the database password in the macOS login keychain under
// service "envsecrets-keepassxc-<vault>". The -U flag acts as an upsert.
func (c *Client) storePassword(ctx context.Context, password string) error {
	svc := "envsecrets-keepassxc-" + c.vault

	cmd := exec.CommandContext(ctx,
		"security", "add-generic-password",
		"-U",
		"-a", currentUser(),
		"-s", svc,
		"-w", password,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("storing password for %q: %w\n%s", svc, err, out)
	}

	return nil
}

// encodeValue percent-encodes characters that act as delimiters in the
// keepassxc-cli stdin prompt protocol (\n and \r), plus the escape character
// itself (%) to make the encoding unambiguous. This allows arbitrary values —
// including multiline strings — to be stored safely.
//
// Only three substitutions are made (in order):
//
//	%  → %25
//	\r → %0D
//	\n → %0A
func encodeValue(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")

	return s
}

// decodeValue reverses encodeValue. Substitutions are applied in reverse order
// so that the literal sequence "%25" is not double-decoded.
//
//	%0A → \n
//	%0D → \r
//	%25 → %
func decodeValue(s string) string {
	s = strings.ReplaceAll(s, "%0A", "\n")
	s = strings.ReplaceAll(s, "%0D", "\r")
	s = strings.ReplaceAll(s, "%25", "%")

	return s
}

// unavailableOrWrap returns ErrUnavailable when err indicates keepassxc-cli
// could not be found in PATH (exec.ErrNotFound), otherwise wraps err with the
// given operation description. Used in methods that call cmd.Output() and need
// to distinguish "binary missing" from other exec failures.
func unavailableOrWrap(op string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("keepassxc %s: %w", op, ErrUnavailable)
	}

	return fmt.Errorf("keepassxc %s: %w", op, err)
}

// classifyError maps keepassxc-cli stderr output to sentinel errors.
func classifyError(key string, out []byte) error {
	msg := strings.ToLower(string(out))

	if strings.Contains(msg, "could not find entry") ||
		strings.Contains(msg, "entry not found") {
		return ErrNotFound
	}

	return fmt.Errorf("keepassxc op on %q: %s", key, strings.TrimSpace(string(out)))
}

// --- access-details file -------------------------------------------------------

func (c *Client) accessFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "envsecrets-"+c.vault+"-keepassxc-access.txt")
}

func (c *Client) writeAccessFile(password string) error {
	path := c.accessFilePath()

	content := fmt.Sprintf(`envsecrets KeePassXC Access Details
====================================
Created: %s

Vault name    : %s
Database file : %s

KEEP THIS FILE SAFE — it contains the password to your KeePassXC database.
Anyone who can read this file can unlock the database and read your secrets.

To open the database in KeePassXC (GUI):
  1. Open KeePassXC
  2. File > Open Database...
  3. Select the database file shown above
  4. Enter the password shown below when prompted

# --- do not edit below this line ---
vault: %s
db-path: %s
password: %s
`,
		time.Now().Format("2006-01-02"),
		c.vault,
		c.dbPath,
		c.vault,
		c.dbPath,
		password,
	)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing access file: %w", err)
	}

	fmt.Fprintf(os.Stderr,
		"info: KeePassXC access details written to %s\n"+
			"      Keep this file safe — it contains your database password.\n",
		path,
	)

	return nil
}

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

// --- small helpers -------------------------------------------------------------

func generatePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func currentUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}

	return u
}
