// Package secrets is the orchestration layer that coordinates reads and writes
// across the macOS Keychain and a configurable durable store.
//
// Write path:  both backends are written; durable store failure is non-fatal
//
//	and only emits a warning so that offline workflows still work.
//
// Read path:   Keychain is tried first (fast, local, always available).
//
//	Falls back to the durable store if the Keychain entry is absent,
//	and writes the result back into Keychain so subsequent reads are fast.
//
// Delete path: both backends are attempted; neither failing blocks the other.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/javorszky/envsecrets/internal/keepassxc"
	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/javorszky/envsecrets/internal/onepassword"
)

// SecretStore is the interface that all backends implement.
// It provides a uniform API for secret storage and retrieval.
type SecretStore interface {
	Available(ctx context.Context) bool
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
	// EnsureVault creates the backend vault (keychain file or durable store
	// database) if it does not already exist, or unlocks/verifies it if it
	// does. Returns (true, nil) when the vault was newly created,
	// (false, nil) when it already existed, or (false, err) on failure.
	EnsureVault(ctx context.Context) (bool, error)
}

// Manager orchestrates secrets across the Keychain cache and a durable store.
type Manager struct {
	kc          SecretStore
	durable     SecretStore
	durableName string    // display name for the durable backend, e.g. "1Password" or "KeePassXC"
	warn        io.Writer // destination for non-fatal warnings (defaults to stderr)
}

// New returns a Manager backed by a dedicated local keychain file and a durable
// backend selected by durableBackend ("1password" or "keepassxc").
//   - keychainVault:  name for the keychain file (~/.local/share/envsecrets/<name>.keychain)
//   - opVault:        1Password vault name (used when durableBackend == "1password")
//   - durableBackend: "1password" (default) or "keepassxc"
//   - kpxcDB:         KeePassXC database stem name (e.g. "envsecrets"); used when durableBackend == "keepassxc"
func New(keychainVault, opVault, durableBackend, kpxcDB string) *Manager {
	var durable SecretStore
	var durableName string

	backend := strings.ToLower(strings.TrimSpace(durableBackend))

	unrecognized := false

	switch backend {
	case "keepassxc":
		durable = keepassxc.New(keychainVault, kpxcDB)
		durableName = "KeePassXC"
	case "", "1password":
		durable = onepassword.New(opVault)
		durableName = "1Password"
	default:
		unrecognized = true
		durable = onepassword.New(opVault)
		durableName = "1Password"
	}

	m := &Manager{
		kc:          keychain.New(keychainVault),
		durable:     durable,
		durableName: durableName,
		warn:        os.Stderr,
	}

	if unrecognized {
		// Route through m.warn so callers who redirect warnings via WithWarningWriter
		// capture this message too, rather than it always going to os.Stderr.
		fmt.Fprintf(m.warn, "warning: unrecognized durable backend %q; falling back to \"1password\"\n", durableBackend)
	}

	return m
}

// NewWithBackends returns a Manager using the provided backend implementations.
// Intended for tests: pass mock or stub implementations of SecretStore
// to exercise the Manager's logic in isolation. durableName is the display
// name used in warning and error messages (e.g. "1Password", "KeePassXC",
// or any label appropriate for the stub).
func NewWithBackends(kc, durable SecretStore, durableName string) *Manager {
	return &Manager{
		kc:          kc,
		durable:     durable,
		durableName: durableName,
		warn:        os.Stderr,
	}
}

// WithWarningWriter overrides where non-fatal warning messages are written.
// Returns the same Manager so calls can be chained.
func (m *Manager) WithWarningWriter(w io.Writer) *Manager {
	m.warn = w
	return m
}

// isDurableNotFound reports whether err represents a "not found" response from
// any supported durable backend.
func isDurableNotFound(err error) bool {
	return errors.Is(err, onepassword.ErrNotFound) || errors.Is(err, keepassxc.ErrNotFound)
}

// Get retrieves a secret by key.
//
//  1. Keychain (fast path)
//  2. Durable store (fallback; result is written back into Keychain)
func (m *Manager) Get(ctx context.Context, key string) (string, error) {
	val, err := m.kc.Get(ctx, key)
	if err == nil {
		return val, nil
	}

	if !errors.Is(err, keychain.ErrNotFound) {
		return "", fmt.Errorf("keychain read: %w", err)
	}

	// Keychain miss — try the durable store.
	if !m.durable.Available(ctx) {
		return "", fmt.Errorf("key %q not in keychain and %s is unavailable", key, m.durableName)
	}

	val, err = m.durable.Get(ctx, key)
	if err != nil {
		if isDurableNotFound(err) {
			return "", fmt.Errorf("key %q not found in keychain or %s", key, m.durableName)
		}

		return "", fmt.Errorf("%s read: %w", strings.ToLower(m.durableName), err)
	}

	// Warm the Keychain so next time we don't need the durable store.
	if kcErr := m.kc.Set(ctx, key, val); kcErr != nil {
		fmt.Fprintf(m.warn, "warning: could not cache %q in keychain: %v\n", key, kcErr)
	}

	return val, nil
}

// Set writes the secret to both backends.
// Durable store failure is treated as a warning — the secret is still stored
// locally in Keychain so the workflow continues offline.
// If either vault does not exist, it is created automatically.
func (m *Manager) Set(ctx context.Context, key, value string) error {
	// Ensure the keychain vault (file) exists before writing.
	// This is fatal: without the keychain file we cannot store anything locally.
	if created, err := m.kc.EnsureVault(ctx); err != nil {
		return fmt.Errorf("keychain vault ensure: %w", err)
	} else if created {
		fmt.Fprintf(m.warn, "info: keychain vault created\n")
	}

	if err := m.kc.Set(ctx, key, value); err != nil {
		return fmt.Errorf("keychain write: %w", err)
	}

	if !m.durable.Available(ctx) {
		fmt.Fprintf(m.warn, "warning: %s unavailable; %q stored in keychain only\n", m.durableName, key)
		return nil
	}

	// Ensure the durable vault exists, creating it if necessary.
	if created, err := m.durable.EnsureVault(ctx); err != nil {
		fmt.Fprintf(m.warn, "warning: could not ensure %s vault: %v\n", m.durableName, err)
	} else if created {
		fmt.Fprintf(m.warn, "info: %s vault created\n", m.durableName)
	}

	if err := m.durable.Set(ctx, key, value); err != nil {
		fmt.Fprintf(m.warn, "warning: %s write failed for %q: %v\n", m.durableName, key, err)
	}

	return nil
}

// Delete removes the secret from both backends. Errors from each are collected
// and returned together; neither failure prevents the other from being attempted.
func (m *Manager) Delete(ctx context.Context, key string) error {
	var errs []error

	if err := m.kc.Delete(ctx, key); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		errs = append(errs, fmt.Errorf("keychain delete: %w", err))
	}

	if m.durable.Available(ctx) {
		if err := m.durable.Delete(ctx, key); err != nil && !isDurableNotFound(err) {
			errs = append(errs, fmt.Errorf("%s delete: %w", strings.ToLower(m.durableName), err))
		}
	} else {
		fmt.Fprintf(m.warn, "warning: %s unavailable; %q removed from keychain only\n", m.durableName, key)
	}

	return errors.Join(errs...)
}

// Sync pulls every item from the durable store and writes it into Keychain.
// This is the bootstrap command you run on a new machine.
func (m *Manager) Sync(ctx context.Context) (synced int, err error) {
	if !m.durable.Available(ctx) {
		return 0, fmt.Errorf("%s is unavailable; cannot sync", m.durableName)
	}

	keys, err := m.durable.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing %s vault: %w", strings.ToLower(m.durableName), err)
	}

	for _, key := range keys {
		val, getErr := m.durable.Get(ctx, key)
		if getErr != nil {
			fmt.Fprintf(m.warn, "warning: skipping %q: %v\n", key, getErr)
			continue
		}

		if setErr := m.kc.Set(ctx, key, val); setErr != nil {
			fmt.Fprintf(m.warn, "warning: could not write %q to keychain: %v\n", key, setErr)
			continue
		}

		synced++
	}

	return synced, nil
}
