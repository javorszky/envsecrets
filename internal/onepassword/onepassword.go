// Package onepassword wraps the `op` CLI to store and retrieve secrets.
//
// All secrets are stored as Login items under a configurable vault.
// The item title is the key name; the secret is stored in the password field.
//
// op CLI docs: https://developer.1password.com/docs/cli/
package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound is returned when a 1Password item does not exist.
var ErrNotFound = errors.New("1password: item not found")

// ErrUnavailable is returned when the `op` binary is not installed or the
// local app is not running / unlocked.
var ErrUnavailable = errors.New("1password: op CLI unavailable")

// Client holds configuration for 1Password operations.
type Client struct {
	// Vault is the 1Password vault name or ID to use (e.g. "Envsecrets", "Work").
	Vault string
}

// New returns a Client targeting the given vault.
func New(vault string) *Client {
	return &Client{Vault: vault}
}

// Available reports whether the op CLI is present and the local 1Password app
// is running and signed in. It does NOT verify vault accessibility.
func (c *Client) Available(ctx context.Context) bool {
	_, err := exec.LookPath("op")
	if err != nil {
		return false
	}

	// `op account list` exits 0 when the app is running and signed in.
	cmd := exec.CommandContext(ctx, "op", "account", "list")

	return cmd.Run() == nil
}

// Get retrieves the password field of the item whose title matches key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/password", c.Vault, key)

	cmd := exec.CommandContext(ctx, "op", "read", ref)

	out, err := cmd.Output()
	if err != nil {
		return "", classifyError(key, err)
	}

	return strings.TrimRight(string(out), "\n"), nil
}

// Set creates or updates an item. It attempts an edit first; if the item does
// not exist it creates a new Login item.
func (c *Client) Set(ctx context.Context, key, value string) error {
	// Try editing an existing item first.
	err := c.edit(ctx, key, value)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrNotFound) {
		return err
	}

	// Item not found — create it.
	return c.create(ctx, key, value)
}

// Delete removes the item whose title matches key from the vault.
func (c *Client) Delete(ctx context.Context, key string) error {
	cmd := exec.CommandContext(ctx,
		"op", "item", "delete",
		"--vault", c.Vault,
		key,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyErrorWithOutput(key, err, out)
	}

	return nil
}

// EnsureVault creates the configured vault in 1Password if it does not already
// exist. Returns (true, nil) when the vault was newly created, (false, nil)
// when it already existed, or (false, err) on failure.
//
// Existence is checked with `op vault get <name>` (exit 0 = exists) rather
// than by parsing `op vault list` output. The list-and-parse approach broke
// silently when the op CLI emitted JSON in a format the parser did not expect,
// causing a new duplicate vault to be created on every store call.
func (c *Client) EnsureVault(ctx context.Context) (bool, error) {
	// `op vault get <name>` exits 0 when the vault exists, non-zero otherwise.
	// No output parsing required.
	if exec.CommandContext(ctx, "op", "vault", "get", c.Vault).Run() == nil {
		return false, nil // vault already exists
	}

	// Vault not found — create it.
	createCmd := exec.CommandContext(ctx, "op", "vault", "create", c.Vault)

	if createOut, createErr := createCmd.CombinedOutput(); createErr != nil {
		return false, fmt.Errorf("1password create vault %q: %w\n%s", c.Vault, createErr, createOut)
	}

	// Write an access-details file to ~/Documents. Best-effort.
	if err := c.writeAccessFile(); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: could not write 1Password access file: %v\n", err)
	}

	return true, nil
}

// List returns all item titles in the vault. Used by the sync command.
func (c *Client) List(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx,
		"op", "item", "list",
		"--vault", c.Vault,
		"--format", "json",
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("1password list vault %q: %w", c.Vault, err)
	}

	// Parse just the titles from the JSON array.
	// We avoid importing encoding/json for a full struct — a simple title
	// extraction is sufficient and keeps the package lean.
	titles, err := ParseTitles(string(out))
	if err != nil {
		return nil, fmt.Errorf("1password list parse: %w", err)
	}

	return titles, nil
}

// --- private helpers --------------------------------------------------------

func (c *Client) create(ctx context.Context, key, value string) error {
	cmd := exec.CommandContext(ctx,
		"op", "item", "create",
		"--category", "Login",
		"--vault", c.Vault,
		"--title", key,
		fmt.Sprintf("password=%s", value),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("1password create %q: %w\n%s", key, err, out)
	}

	return nil
}

func (c *Client) edit(ctx context.Context, key, value string) error {
	cmd := exec.CommandContext(ctx,
		"op", "item", "edit",
		"--vault", c.Vault,
		key,
		fmt.Sprintf("password=%s", value),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyErrorWithOutput(key, err, out)
	}

	return nil
}

func classifyError(key string, err error) error {
	return classifyErrorWithOutput(key, err, nil)
}

func classifyErrorWithOutput(key string, err error, out []byte) error {
	msg := strings.ToLower(string(out))

	if strings.Contains(msg, "isn't an item") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no item") ||
		strings.Contains(msg, "could not find") {
		return ErrNotFound
	}

	if strings.Contains(msg, "not currently signed in") ||
		strings.Contains(msg, "connect to the 1password app") {
		return fmt.Errorf("%w: %s", ErrUnavailable, msg)
	}

	return fmt.Errorf("1password op on %q: %w", key, err)
}

// --- access-details file -----------------------------------------------------

// accessFilePath returns the path of the access-details file written when the
// 1Password vault is first created.
func (c *Client) accessFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "envsecrets-"+c.Vault+"-1password-access.txt")
}

// writeAccessFile writes a plaintext file to ~/Documents explaining how to
// locate the 1Password vault without the CLI.
func (c *Client) writeAccessFile() error {
	path := c.accessFilePath()

	content := fmt.Sprintf(`envsecrets 1Password Vault Access Details
==========================================
Created: %s

Vault name: %s

Your envsecrets secrets are stored in the "%s" vault in 1Password.

To access your secrets without the envsecrets CLI:
  1. Open 1Password
  2. Select the "%s" vault from the sidebar
  3. Secrets are stored as Login items; the value is in the password field
`,
		time.Now().Format("2006-01-02"),
		c.Vault,
		c.Vault,
		c.Vault,
	)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing access file: %w", err)
	}

	fmt.Fprintf(os.Stderr,
		"info: 1Password vault access details written to %s\n",
		path,
	)

	return nil
}

// ParseVaultNames extracts vault names from the JSON array returned by
// `op vault list --format json`.
// The expected shape is: [{"id":"...","name":"MyVault",...}, ...]
// Returns nil on empty or unparseable input.
func ParseVaultNames(jsonStr string) []string {
	var vaults []struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &vaults); err != nil {
		return nil
	}

	var names []string

	for _, v := range vaults {
		if v.Name != "" {
			names = append(names, v.Name)
		}
	}

	return names
}

// ParseTitles extracts item titles from the JSON array returned by `op item list`.
// The expected shape is: [{"id":"...","title":"MY_KEY","vault":{...}}, ...]
// Returns (nil, nil) on empty input; returns an error on malformed JSON.
func ParseTitles(jsonStr string) ([]string, error) {
	var items []struct {
		Title string `json:"title"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		// Treat empty output as an empty result rather than an error — `op`
		// can return nothing when a vault has no items.
		if strings.TrimSpace(jsonStr) == "" {
			return nil, nil
		}

		return nil, fmt.Errorf("parsing items JSON: %w", err)
	}

	var titles []string

	for _, item := range items {
		if item.Title != "" {
			titles = append(titles, item.Title)
		}
	}

	return titles, nil
}
