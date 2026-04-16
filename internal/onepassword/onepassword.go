// Package onepassword wraps the `op` CLI to store and retrieve secrets.
//
// All secrets are stored as Login items under a configurable vault.
// The item title is the key name; the secret is stored in the password field.
//
// op CLI docs: https://developer.1password.com/docs/cli/
package onepassword

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotFound is returned when a 1Password item does not exist.
var ErrNotFound = errors.New("1password: item not found")

// ErrUnavailable is returned when the `op` binary is not installed or the
// local app is not running / unlocked.
var ErrUnavailable = errors.New("1password: op CLI unavailable")

// Client holds configuration for 1Password operations.
type Client struct {
	// Vault is the 1Password vault name or ID to use (e.g. "Private", "Work").
	Vault string
}

// New returns a Client targeting the given vault.
func New(vault string) *Client {
	return &Client{Vault: vault}
}

// Available reports whether the op CLI is present and the local 1Password app
// is running and signed in. It does NOT verify vault accessibility.
func (c *Client) Available() bool {
	_, err := exec.LookPath("op")
	if err != nil {
		return false
	}

	// `op account list` exits 0 when the app is running and signed in.
	return exec.Command("op", "account", "list").Run() == nil
}

// Get retrieves the password field of the item whose title matches key.
func (c *Client) Get(key string) (string, error) {
	ref := fmt.Sprintf("op://%s/%s/password", c.Vault, key)

	out, err := exec.Command("op", "read", ref).Output()
	if err != nil {
		return "", c.classifyError(key, err)
	}

	return strings.TrimRight(string(out), "\n"), nil
}

// Set creates or updates an item. It attempts an edit first; if the item does
// not exist it creates a new Login item.
func (c *Client) Set(key, value string) error {
	// Try editing an existing item first.
	err := c.edit(key, value)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrNotFound) {
		return err
	}

	// Item not found — create it.
	return c.create(key, value)
}

// Delete removes the item whose title matches key from the vault.
func (c *Client) Delete(key string) error {
	out, err := exec.Command(
		"op", "item", "delete",
		"--vault", c.Vault,
		key,
	).CombinedOutput()
	if err != nil {
		return c.classifyErrorWithOutput(key, err, out)
	}

	return nil
}

// List returns all item titles in the vault. Used by the sync command.
func (c *Client) List() ([]string, error) {
	out, err := exec.Command(
		"op", "item", "list",
		"--vault", c.Vault,
		"--format", "json",
	).Output()
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

func (c *Client) create(key, value string) error {
	out, err := exec.Command(
		"op", "item", "create",
		"--category", "Login",
		"--vault", c.Vault,
		"--title", key,
		fmt.Sprintf("password=%s", value),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("1password create %q: %w\n%s", key, err, out)
	}

	return nil
}

func (c *Client) edit(key, value string) error {
	out, err := exec.Command(
		"op", "item", "edit",
		"--vault", c.Vault,
		key,
		fmt.Sprintf("password=%s", value),
	).CombinedOutput()
	if err != nil {
		return c.classifyErrorWithOutput(key, err, out)
	}

	return nil
}

func (c *Client) classifyError(key string, err error) error {
	return c.classifyErrorWithOutput(key, err, nil)
}

func (c *Client) classifyErrorWithOutput(key string, err error, out []byte) error {
	msg := strings.ToLower(string(out))

	if strings.Contains(msg, "isn't an item") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no item") {
		return ErrNotFound
	}

	if strings.Contains(msg, "not currently signed in") ||
		strings.Contains(msg, "connect to the 1password app") {
		return fmt.Errorf("%w: %s", ErrUnavailable, msg)
	}

	return fmt.Errorf("1password op on %q: %w", key, err)
}

// ParseTitles extracts item titles from the JSON array returned by `op item list`.
// The expected shape is: [{"id":"...","title":"MY_KEY","vault":{...}}, ...]
// It avoids importing encoding/json to keep the package lean.
func ParseTitles(jsonStr string) ([]string, error) {
	// Manual scan instead of encoding/json to keep the package lean.
	var titles []string

	// Split on `"title":"` and extract the value up to the next `"`.
	parts := strings.Split(jsonStr, `"title":"`)
	for i, p := range parts {
		if i == 0 {
			continue // everything before the first match
		}

		end := strings.Index(p, `"`)
		if end < 0 {
			continue
		}

		titles = append(titles, p[:end])
	}

	return titles, nil
}
