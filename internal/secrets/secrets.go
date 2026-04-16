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
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/javorszky/envsecrets/internal/onepassword"
)

// SecretStore is the interface that both backends (keychain and 1Password)
// implement. It provides a uniform API for secret storage and retrieval.
type SecretStore interface {
	Available(ctx context.Context) bool
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
}

// Manager orchestrates secrets across Keychain and 1Password.
type Manager struct {
	kc   SecretStore
	op   SecretStore
	warn io.Writer // destination for non-fatal warnings (defaults to stderr)
}

// New returns a Manager backed by macOS Keychain and the given 1Password vault.
func New(vault string) *Manager {
	return &Manager{
		kc:   keychain.New(vault),
		op:   onepassword.New(vault),
		warn: os.Stderr,
	}
}

// NewWithBackends returns a Manager using the provided backend implementations.
// Intended for tests: pass mock or stub implementations of SecretStore
// to exercise the Manager's logic in isolation.
func NewWithBackends(kc, op SecretStore) *Manager {
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
func (m *Manager) Get(ctx context.Context, key string) (string, error) {
	val, err := m.kc.Get(ctx, key)
	if err == nil {
		return val, nil
	}

	if !errors.Is(err, keychain.ErrNotFound) {
		return "", fmt.Errorf("keychain read: %w", err)
	}

	// Keychain miss — try 1Password.
	if !m.op.Available(ctx) {
		return "", fmt.Errorf("key %q not in keychain and 1Password is unavailable", key)
	}

	val, err = m.op.Get(ctx, key)
	if err != nil {
		if errors.Is(err, onepassword.ErrNotFound) {
			return "", fmt.Errorf("key %q not found in keychain or 1Password", key)
		}

		return "", fmt.Errorf("1password read: %w", err)
	}

	// Warm the Keychain so next time we don't need 1Password.
	if kcErr := m.kc.Set(ctx, key, val); kcErr != nil {
		_, _ = fmt.Fprintf(m.warn, "warning: could not cache %q in keychain: %v\n", key, kcErr)
	}

	return val, nil
}

// Set writes the secret to both backends.
// 1Password failure is treated as a warning — the secret is still stored
// locally in Keychain so the workflow continues offline.
func (m *Manager) Set(ctx context.Context, key, value string) error {
	if err := m.kc.Set(ctx, key, value); err != nil {
		return fmt.Errorf("keychain write: %w", err)
	}

	if !m.op.Available(ctx) {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password unavailable; %q stored in keychain only\n", key)
		return nil
	}

	if err := m.op.Set(ctx, key, value); err != nil {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password write failed for %q: %v\n", key, err)
	}

	return nil
}

// Update is an alias for Set — the distinction is semantic at the CLI layer.
func (m *Manager) Update(ctx context.Context, key, value string) error {
	return m.Set(ctx, key, value)
}

// Delete removes the secret from both backends. Errors from each are collected
// and returned together; neither failure prevents the other from being attempted.
func (m *Manager) Delete(ctx context.Context, key string) error {
	var errs []error

	if err := m.kc.Delete(ctx, key); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		errs = append(errs, fmt.Errorf("keychain delete: %w", err))
	}

	if m.op.Available(ctx) {
		if err := m.op.Delete(ctx, key); err != nil && !errors.Is(err, onepassword.ErrNotFound) {
			errs = append(errs, fmt.Errorf("1password delete: %w", err))
		}
	} else {
		_, _ = fmt.Fprintf(m.warn, "warning: 1Password unavailable; %q removed from keychain only\n", key)
	}

	return errors.Join(errs...)
}

// Sync pulls every item from the 1Password vault and writes it into Keychain.
// This is the bootstrap command you run on a new machine.
func (m *Manager) Sync(ctx context.Context) (synced int, err error) {
	if !m.op.Available(ctx) {
		return 0, fmt.Errorf("1Password is unavailable; cannot sync")
	}

	keys, err := m.op.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing 1password vault: %w", err)
	}

	for _, key := range keys {
		val, getErr := m.op.Get(ctx, key)
		if getErr != nil {
			_, _ = fmt.Fprintf(m.warn, "warning: skipping %q: %v\n", key, getErr)
			continue
		}

		if setErr := m.kc.Set(ctx, key, val); setErr != nil {
			_, _ = fmt.Fprintf(m.warn, "warning: could not write %q to keychain: %v\n", key, setErr)
			continue
		}

		synced++
	}

	return synced, nil
}
