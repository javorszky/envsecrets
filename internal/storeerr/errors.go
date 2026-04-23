// Package storeerr defines sentinel errors shared across all secret-store
// backends. Backends import this package to avoid a cycle: internal/secrets
// (the orchestration layer) also imports it, so errors can be compared
// uniformly with a single errors.Is call.
package storeerr

import "errors"

// ErrNotFound is returned by any backend when a requested secret key does not
// exist in that backend's store.
var ErrNotFound = errors.New("secret not found")
