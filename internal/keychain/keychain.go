// Package keychain wraps the macOS `security` CLI to store and retrieve
// generic passwords from the user's login keychain.
//
// All entries are stored with:
//   - account:  current $USER
//   - service:  the key name as-is (callers should namespace, e.g. "myapp_DB_PASSWORD")
//
// Use Client to interact with the keychain. The zero value is ready to use.
package keychain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNotFound is returned when a keychain entry does not exist.
var ErrNotFound = errors.New("keychain: entry not found")

// Client is a handle for macOS Keychain operations.
// The zero value is ready to use; it satisfies the Keychain interface expected
// by internal/secrets.Manager.
type Client struct{}

// Get retrieves the secret for the given service key.
// Returns ErrNotFound if the entry does not exist.
func (Client) Get(service string) (string, error) {
	out, err := exec.Command(
		"security",
		"find-generic-password",
		"-a", currentUser(),
		"-s", service,
		"-w",
	).Output()
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
func (c Client) Set(service, value string) error {
	// Best-effort delete; ignore errors (entry may not exist yet).
	_ = c.remove(service)

	cmd := exec.Command(
		"security",
		"add-generic-password",
		"-a", currentUser(),
		"-s", service,
		"-w", value,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keychain set %q: %w\n%s", service, err, out)
	}

	return nil
}

// Delete removes the keychain entry for the given service key.
// Returns ErrNotFound if the entry does not exist.
func (c Client) Delete(service string) error {
	return c.remove(service)
}

// remove is the internal implementation for deleting a keychain entry.
func (Client) remove(service string) error {
	cmd := exec.Command(
		"security",
		"delete-generic-password",
		"-a", currentUser(),
		"-s", service,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return ErrNotFound
		}

		return fmt.Errorf("keychain delete %q: %w\n%s", service, err, out)
	}

	return nil
}

// currentUser returns the OS username, falling back to LOGNAME.
func currentUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}

	return u
}
