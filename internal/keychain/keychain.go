// Package keychain wraps the macOS `security` CLI to store and retrieve
// generic passwords from the user's login keychain.
//
// All entries are stored with:
//   - account:  current $USER
//   - service:  the key name as-is (callers should namespace, e.g. "myapp_DB_PASSWORD")
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
func (Client) Get(service string) (string, error) { return Get(service) }

// Set stores or overwrites a secret.
func (Client) Set(service, value string) error { return Set(service, value) }

// Delete removes the keychain entry for the given service key.
// Returns ErrNotFound if the entry does not exist.
func (Client) Delete(service string) error { return Delete(service) }

func user() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}

	return u
}

// Get retrieves the secret for the given service key.
// Returns ErrNotFound if the entry does not exist.
func Get(service string) (string, error) {
	out, err := exec.Command(
		"security",
		"find-generic-password",
		"-a", user(),
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
func Set(service, value string) error {
	// Best-effort delete; ignore errors (entry may not exist yet).
	_ = delete(service)

	cmd := exec.Command(
		"security",
		"add-generic-password",
		"-a", user(),
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
func Delete(service string) error {
	return delete(service)
}

func delete(service string) error {
	cmd := exec.Command(
		"security",
		"delete-generic-password",
		"-a", user(),
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
