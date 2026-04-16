package secrets_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/javorszky/envsecrets/internal/onepassword"
	"github.com/javorszky/envsecrets/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stub backends
// ---------------------------------------------------------------------------

// stubKC is an in-memory Keychain stub.
// getErr / setErr / deleteErr, when non-nil, are returned for every call of
// that operation (use nil fields for success paths).
// To simulate a "key not found" on Get, simply omit the key from data.
type stubKC struct {
	data      map[string]string
	getErr    error
	setErr    error
	deleteErr error
}

func newStubKC(initial map[string]string) *stubKC {
	data := make(map[string]string)
	for k, v := range initial {
		data[k] = v
	}
	return &stubKC{data: data}
}

func (s *stubKC) Get(key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	v, ok := s.data[key]
	if !ok {
		return "", keychain.ErrNotFound
	}
	return v, nil
}

func (s *stubKC) Set(key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.data[key] = value
	return nil
}

func (s *stubKC) Delete(key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.data[key]; !ok {
		return keychain.ErrNotFound
	}
	delete(s.data, key)
	return nil
}

// stubOP is an in-memory 1Password stub.
type stubOP struct {
	avail     bool
	data      map[string]string
	getErr    error
	setErr    error
	deleteErr error
	listErr   error
}

func newStubOP(avail bool, initial map[string]string) *stubOP {
	data := make(map[string]string)
	for k, v := range initial {
		data[k] = v
	}
	return &stubOP{avail: avail, data: data}
}

func (s *stubOP) Available() bool { return s.avail }

func (s *stubOP) Get(key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	v, ok := s.data[key]
	if !ok {
		return "", onepassword.ErrNotFound
	}
	return v, nil
}

func (s *stubOP) Set(key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.data[key] = value
	return nil
}

func (s *stubOP) Delete(key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.data[key]; !ok {
		return onepassword.ErrNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *stubOP) List() ([]string, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// newMgr wires up a Manager with stub backends and a warning capture buffer.
func newMgr(kc *stubKC, op *stubOP) (*secrets.Manager, *strings.Builder) {
	warn := &strings.Builder{}
	return secrets.NewWithBackends(kc, op).WithWarningWriter(warn), warn
}

// ---------------------------------------------------------------------------
// Manager.Get
// ---------------------------------------------------------------------------

func TestManager_Get(t *testing.T) {
	t.Parallel()

	errDisk := errors.New("disk I/O error")
	errOpIO := errors.New("op subprocess crashed")

	tests := []struct {
		name            string
		kcData          map[string]string
		kcGetErr        error
		kcSetErr        error // injected for cache-write step
		opAvail         bool
		opData          map[string]string
		opGetErr        error
		key             string
		wantVal         string
		wantErr         bool
		wantErrContains string
		wantWarning     string
		wantKCAfter     map[string]string // non-nil → assert keychain state
	}{
		{
			name:        "keychain hit — returns immediately",
			kcData:      map[string]string{"MY_KEY": "kc_value"},
			key:         "MY_KEY",
			wantVal:     "kc_value",
			wantKCAfter: map[string]string{"MY_KEY": "kc_value"},
		},
		{
			name:        "keychain miss, 1password hit — returns value and caches in keychain",
			kcData:      map[string]string{},
			opAvail:     true,
			opData:      map[string]string{"MY_KEY": "op_value"},
			key:         "MY_KEY",
			wantVal:     "op_value",
			wantKCAfter: map[string]string{"MY_KEY": "op_value"},
		},
		{
			name:        "keychain miss, 1password hit, keychain cache fails — returns value with warning",
			kcData:      map[string]string{},
			kcSetErr:    errors.New("keychain full"),
			opAvail:     true,
			opData:      map[string]string{"MY_KEY": "op_value"},
			key:         "MY_KEY",
			wantVal:     "op_value",
			wantWarning: `could not cache "MY_KEY" in keychain`,
		},
		{
			name:            "keychain miss, 1password unavailable — error",
			kcData:          map[string]string{},
			opAvail:         false,
			key:             "MY_KEY",
			wantErr:         true,
			wantErrContains: "1Password is unavailable",
		},
		{
			name:            "keychain miss, 1password available but key absent — error",
			kcData:          map[string]string{},
			opAvail:         true,
			opData:          map[string]string{},
			key:             "MY_KEY",
			wantErr:         true,
			wantErrContains: `not found in keychain or 1Password`,
		},
		{
			name:            "keychain miss, 1password returns unexpected error",
			kcData:          map[string]string{},
			opAvail:         true,
			opGetErr:        errOpIO,
			key:             "MY_KEY",
			wantErr:         true,
			wantErrContains: "1password read",
		},
		{
			name:            "keychain returns non-ErrNotFound error",
			kcGetErr:        errDisk,
			key:             "MY_KEY",
			wantErr:         true,
			wantErrContains: "keychain read",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kc := newStubKC(tc.kcData)
			kc.getErr = tc.kcGetErr
			kc.setErr = tc.kcSetErr

			op := newStubOP(tc.opAvail, tc.opData)
			op.getErr = tc.opGetErr

			mgr, warnBuf := newMgr(kc, op)

			val, err := mgr.Get(tc.key)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantVal, val)

			if tc.wantWarning != "" {
				assert.Contains(t, warnBuf.String(), tc.wantWarning)
			} else {
				assert.Empty(t, warnBuf.String())
			}

			if tc.wantKCAfter != nil {
				assert.Equal(t, tc.wantKCAfter, kc.data)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.Set
// ---------------------------------------------------------------------------

func TestManager_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		kcSetErr        error
		opAvail         bool
		opSetErr        error
		key             string
		value           string
		wantErr         bool
		wantErrContains string
		wantWarning     string
		wantKCAfter     map[string]string
		wantOPAfter     map[string]string
	}{
		{
			name:        "both backends succeed",
			opAvail:     true,
			key:         "DB_PASS",
			value:       "secret123",
			wantKCAfter: map[string]string{"DB_PASS": "secret123"},
			wantOPAfter: map[string]string{"DB_PASS": "secret123"},
		},
		{
			name:        "1password unavailable — stores in keychain only, warns",
			opAvail:     false,
			key:         "DB_PASS",
			value:       "secret123",
			wantKCAfter: map[string]string{"DB_PASS": "secret123"},
			wantWarning: "1Password unavailable",
		},
		{
			name:        "1password write fails — keychain ok, warns",
			opAvail:     true,
			opSetErr:    errors.New("vault locked"),
			key:         "DB_PASS",
			value:       "secret123",
			wantKCAfter: map[string]string{"DB_PASS": "secret123"},
			wantWarning: "1Password write failed",
		},
		{
			name:            "keychain write fails — error returned, 1password not tried",
			kcSetErr:        errors.New("keychain full"),
			opAvail:         true,
			key:             "DB_PASS",
			value:           "secret123",
			wantErr:         true,
			wantErrContains: "keychain write",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kc := newStubKC(nil)
			kc.setErr = tc.kcSetErr

			op := newStubOP(tc.opAvail, nil)
			op.setErr = tc.opSetErr

			mgr, warnBuf := newMgr(kc, op)

			err := mgr.Set(tc.key, tc.value)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				return
			}

			require.NoError(t, err)

			if tc.wantWarning != "" {
				assert.Contains(t, warnBuf.String(), tc.wantWarning)
			} else {
				assert.Empty(t, warnBuf.String())
			}

			if tc.wantKCAfter != nil {
				assert.Equal(t, tc.wantKCAfter, kc.data)
			}
			if tc.wantOPAfter != nil {
				assert.Equal(t, tc.wantOPAfter, op.data)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.Update
// ---------------------------------------------------------------------------

func TestManager_Update(t *testing.T) {
	t.Parallel()

	// Update is a semantic alias for Set — one case verifies the delegation.
	kc := newStubKC(map[string]string{"KEY": "old"})
	op := newStubOP(true, map[string]string{"KEY": "old"})
	mgr, _ := newMgr(kc, op)

	require.NoError(t, mgr.Update("KEY", "new"))
	assert.Equal(t, "new", kc.data["KEY"])
	assert.Equal(t, "new", op.data["KEY"])
}

// ---------------------------------------------------------------------------
// Manager.Delete
// ---------------------------------------------------------------------------

func TestManager_Delete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		kcData          map[string]string
		kcDeleteErr     error
		opAvail         bool
		opData          map[string]string
		opDeleteErr     error
		key             string
		wantErr         bool
		wantErrContains string
		wantWarning     string
		wantKCAfter     map[string]string
		wantOPAfter     map[string]string
	}{
		{
			name:        "key in both backends — both deleted",
			kcData:      map[string]string{"K": "v"},
			opAvail:     true,
			opData:      map[string]string{"K": "v"},
			key:         "K",
			wantKCAfter: map[string]string{},
			wantOPAfter: map[string]string{},
		},
		{
			name:        "key in keychain only — keychain deleted, 1password ErrNotFound ignored",
			kcData:      map[string]string{"K": "v"},
			opAvail:     true,
			opData:      map[string]string{},
			key:         "K",
			wantKCAfter: map[string]string{},
			wantOPAfter: map[string]string{},
		},
		{
			name:        "key in 1password only — keychain ErrNotFound ignored, 1password deleted",
			kcData:      map[string]string{},
			opAvail:     true,
			opData:      map[string]string{"K": "v"},
			key:         "K",
			wantKCAfter: map[string]string{},
			wantOPAfter: map[string]string{},
		},
		{
			name:        "key in neither — both ErrNotFound ignored, success",
			kcData:      map[string]string{},
			opAvail:     true,
			opData:      map[string]string{},
			key:         "K",
			wantKCAfter: map[string]string{},
			wantOPAfter: map[string]string{},
		},
		{
			name:        "1password unavailable — deleted from keychain only, warning emitted",
			kcData:      map[string]string{"K": "v"},
			opAvail:     false,
			key:         "K",
			wantKCAfter: map[string]string{},
			wantWarning: "removed from keychain only",
		},
		{
			name:            "keychain delete returns unexpected error — included in result",
			kcDeleteErr:     errors.New("disk failure"),
			opAvail:         true,
			opData:          map[string]string{},
			key:             "K",
			wantErr:         true,
			wantErrContains: "keychain delete",
		},
		{
			name:            "1password delete returns unexpected error — included in result",
			kcData:          map[string]string{},
			opAvail:         true,
			opDeleteErr:     errors.New("vault unavailable"),
			key:             "K",
			wantErr:         true,
			wantErrContains: "1password delete",
		},
		{
			name:            "both backends return unexpected errors — both included via errors.Join",
			kcDeleteErr:     errors.New("kc error"),
			opAvail:         true,
			opDeleteErr:     errors.New("op error"),
			key:             "K",
			wantErr:         true,
			wantErrContains: "keychain delete",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kc := newStubKC(tc.kcData)
			kc.deleteErr = tc.kcDeleteErr

			op := newStubOP(tc.opAvail, tc.opData)
			op.deleteErr = tc.opDeleteErr

			mgr, warnBuf := newMgr(kc, op)

			err := mgr.Delete(tc.key)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				return
			}

			require.NoError(t, err)

			if tc.wantWarning != "" {
				assert.Contains(t, warnBuf.String(), tc.wantWarning)
			} else {
				assert.Empty(t, warnBuf.String())
			}

			if tc.wantKCAfter != nil {
				assert.Equal(t, tc.wantKCAfter, kc.data)
			}
			if tc.wantOPAfter != nil {
				assert.Equal(t, tc.wantOPAfter, op.data)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Manager.Sync
// ---------------------------------------------------------------------------

func TestManager_Sync(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		opAvail         bool
		opData          map[string]string
		opListErr       error
		opGetErr        error
		kcSetErr        error
		wantSynced      int
		wantErr         bool
		wantErrContains string
		wantWarning     string
		wantKCKeys      []string // keys expected in keychain after sync
	}{
		{
			name:       "all items sync successfully",
			opAvail:    true,
			opData:     map[string]string{"K1": "v1", "K2": "v2", "K3": "v3"},
			wantSynced: 3,
			wantKCKeys: []string{"K1", "K2", "K3"},
		},
		{
			name:       "vault is empty — zero synced",
			opAvail:    true,
			opData:     map[string]string{},
			wantSynced: 0,
		},
		{
			name:            "1password unavailable — error",
			opAvail:         false,
			wantErr:         true,
			wantErrContains: "1Password is unavailable",
		},
		{
			name:            "List returns error — propagated",
			opAvail:         true,
			opListErr:       errors.New("network timeout"),
			wantErr:         true,
			wantErrContains: "listing 1password vault",
		},
		{
			name:        "Get fails for all items — zero synced, warning per item",
			opAvail:     true,
			opData:      map[string]string{"K1": "v1"},
			opGetErr:    errors.New("item corrupted"),
			wantSynced:  0,
			wantWarning: `skipping "K1"`,
		},
		{
			name:        "keychain Set fails for all items — zero synced, warning per item",
			opAvail:     true,
			opData:      map[string]string{"K1": "v1"},
			kcSetErr:    errors.New("keychain locked"),
			wantSynced:  0,
			wantWarning: `could not write "K1" to keychain`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			kc := newStubKC(nil)
			kc.setErr = tc.kcSetErr

			op := newStubOP(tc.opAvail, tc.opData)
			op.listErr = tc.opListErr
			op.getErr = tc.opGetErr

			mgr, warnBuf := newMgr(kc, op)

			synced, err := mgr.Sync()

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSynced, synced)

			if tc.wantWarning != "" {
				assert.Contains(t, warnBuf.String(), tc.wantWarning)
			} else {
				assert.Empty(t, warnBuf.String())
			}

			if tc.wantKCKeys != nil {
				assert.ElementsMatch(t, tc.wantKCKeys, keysOf(kc.data))
			}
		})
	}
}

// keysOf returns the map keys as an unsorted slice.
func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
