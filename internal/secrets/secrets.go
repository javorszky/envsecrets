// Package secrets is the orchestration layer that coordinates reads and writes
// across the macOS Keychain and 1Password.
//
// Write path:  both backends are written; 1Password failure is non-fatal and
//
//	only emits a warning so that offline / no-op workflows still work.
//
// Read path:   Keychain is tried first (fast, local, always available).
//
//	Falls back to 1Password if the Keychain entry is absent, and
//	writes the result back into Keychain so subsequent reads are fast.
//
// Delete path: both backends are attempted; neither failing blocks the other.
package secrets

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/javorszky/envsecrets/internal/onepassword"
)

// Keychain is the interface for the local, always-available secret store.
// keychain.Client satisfies this interface.
type Keychain interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
}

// OnePassword is the interface for the remote, durable secret store.
// *onepassword.Client satisfies this interface.
type OnePassword interface {
	Available() bool
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	List() ([]string, error)
}

// Manager orchestrates secrets across Keychain and 1Password.
type Manager struct {
	kc   Keychain
	op   OnePassword
	warn io.Writer // destination for non-fatal warnings (defaults to stderr)
}

// New returns a Manager backed by macOS Keychain and the given 1Password vault.
func New(vault string) *Manager {
	return &Manager{
		kc:   keychain.Client{},
		op:   onepassword.New(vault),
		warn: os.Stderr,
	}
}

// NewWithBackends returns a Manager using the provided backend implementations.
// Intended for tests: pass mock or stub implementations of Keychain and
// OnePassword to exercise the Manager's logic in isolation.
func NewWithBackends(kc Keychain, op OnePassword) *Manager {
	return &Manager{
		kc:   kc,
		op:   op,
		warn: os.Stderr,
	}
}

// WithWarningWriter overrides where non-fatal warning messages are written.
// Returns the same Manager so calls can be chained.
func (m *Manager) WithWarningWriter(w io.Writer) *Manager {
	m.warn = w
	return m
}

// Get retrieves a secret by key.
//
//  1. Keychain (fast path)
//  2. 1Password (fallback; result is written back into Keychain)
func (m *Manager) Get(key string) (string, error) {
	val, err := m.kc.Get(key)
	if err == nil {
		return val, nil
	}

	if !errors.Is(err, keychain.ErrNotFound) {
		return "", fmt.Errorf("keychain read: %w", err)
	}

	// Keychain miss — try 1Password.
	if !m.op.Available() {
		return "", fmt.Errorf("key %q not in keychain and 1Password is unavailable", key)
	}

	val, err = m.op.Get(key)
	if err != nil {
		if errors.Is(err, onepassword.ErrNotFound) {
			return "", fmt.Errorf("key %q not found in keychain or 1Password", key)
		}

		return "", fmt.Errorf("1password read: %w", err)
	}

	// Warm the Keychain so next time we don't need 1Password.
	if kcErr := m.kc.Set(key, val); kcErr != nil {
		_, _ = fmt.Fprintf(m.warn, "warning: could not cache %q in keychain: %v\n", key, kcErr)
	}

	return val, nil
}

// Set writes the secret to both backends.
// 1Password failure is treated as a warning — the secret is still stored
// locally in Keychain so the workflow continues offline.
func (m *Manager) Set(key, value string) error {
	if err := m.kc.Set(key, value); err != nil {
		return fmt.Errorf("keychain write: %w", err)
	}

	if !m.op.Available() {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password unavailable; %q stored in keychain only\n", key)
		return nil
	}

	if err := m.op.Set(key, value); err != nil {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password write failed for %q: %v\n", key, err)
	}

	return nil
}

// Update is an alias for Set — the distinction is semantic at the CLI layer.
func (m *Manager) Update(key, value string) error {
	return m.Set(key, value)
}

// Delete removes the secret from both backends. Errors from each are collected
// and returned together; neither failure prevents the other from being attempted.
func (m *Manager) Delete(key string) error {
	var errs []error

	if err := m.kc.Delete(key); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		errs = append(errs, fmt.Errorf("keychain delete: %w", err))
	}

	if m.op.Available() {
		if err := m.op.Delete(key); err != nil && !errors.Is(err, onepassword.ErrNotFound) {
			errs = append(errs, fmt.Errorf("1password delete: %w", err))
		}
	} else {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password unavailable; %q removed from keychain only\n", key)
	}

	return errors.Join(errs...)
}

// Sync pulls every item from the 1Password vault and writes it into Keychain.
// This is the bootstrap command you run on a new machine.
func (m *Manager) Sync() (synced int, err error) {
	if !m.op.Available() {
		return 0, fmt.Errorf("1Password is unavailable; cannot sync")
	}

	keys, err := m.op.List()
	if err != nil {
		return 0, fmt.Errorf("listing 1password vault: %w", err)
	}

	for _, key := range keys {
		val, getErr := m.op.Get(key)
		if getErr != nil {
			_, _ = fmt.Fprintf(m.warn, "warning: skipping %q: %v\n", key, getErr)
			continue
		}

		if setErr := m.kc.Set(key, val); setErr != nil {
			_, _ = fmt.Fprintf(m.warn, "warning: could not write %q to keychain: %v\n", key, setErr)
			continue
		}

		synced++
	}

	return synced, nil
}
