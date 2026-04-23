// Package loginkc wraps the macOS login keychain as a credential store
// for vault passwords. It is used by internal/keychain and
// internal/keepassxc to avoid duplicating the store/read/fallback pattern
// for backend passwords that are kept in the user's login keychain.
package loginkc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Store upserts a password into the macOS login keychain under service.
// The -U flag makes add-generic-password act as an upsert so a stale
// entry from a previous installation never causes a "duplicate item" failure.
func Store(ctx context.Context, service, password string) error {
	cmd := exec.CommandContext(ctx,
		"security", "add-generic-password",
		"-U",
		"-a", CurrentUser(),
		"-s", service,
		"-w", password,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("storing password for %q: %w\n%s", service, err, out)
	}

	return nil
}

// ReadWithFallback retrieves a password for service from the macOS login
// keychain. On exit code 44 ("item not found") it calls fallback to recover
// the password (e.g. from an access file on disk), then restores the
// login-keychain entry so subsequent calls are fast again.
//
// Any other failure — context cancellation, binary not in PATH, permission
// denied, keychain locked — is returned immediately so callers see the real
// cause rather than a confusing access-file error.
func ReadWithFallback(ctx context.Context, service string, fallback func() (string, error)) (string, error) {
	cmd := exec.CommandContext(ctx,
		"security", "find-generic-password",
		"-a", CurrentUser(),
		"-s", service,
		"-w",
	)

	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSuffix(string(out), "\n"), nil
	}

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return "", fmt.Errorf("reading password for %q from login keychain: %w", service, err)
	}

	if exitErr.ExitCode() != 44 {
		if msg := strings.TrimSpace(string(exitErr.Stderr)); msg != "" {
			return "", fmt.Errorf("reading password for %q from login keychain: %w\n%s", service, err, msg)
		}

		return "", fmt.Errorf("reading password for %q from login keychain: %w", service, err)
	}

	// Login-keychain item not found — try the fallback source.
	pw, fileErr := fallback()
	if fileErr != nil {
		return "", fmt.Errorf(
			"reading password for %q from login keychain (%w) and from access file (%v)",
			service, err, fileErr,
		)
	}

	// Restore the login-keychain entry so next time is seamless.
	_ = Store(ctx, service, pw)

	return pw, nil
}

// GeneratePassword returns a 64-character hex string derived from 32
// cryptographically random bytes.
func GeneratePassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// CurrentUser returns the OS username ($USER), falling back to $LOGNAME.
func CurrentUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}

	return u
}
