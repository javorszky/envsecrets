package keepassxc_test

import (
	"errors"
	"testing"

	"github.com/javorszky/envsecrets/internal/keepassxc"
	"github.com/stretchr/testify/assert"
)

func TestValidateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		// --- valid keys ---
		{name: "plain env var name", key: "DB_PASSWORD", wantErr: false},
		{name: "name with hyphens", key: "my-secret-key", wantErr: false},
		{name: "name with dots", key: "my.secret.key", wantErr: false},
		{name: "unicode letters", key: "Ключ", wantErr: false},

		// --- empty ---
		{name: "empty key", key: "", wantErr: true},

		// --- slash (KeePassXC path separator) ---
		{name: "slash in middle", key: "MYAPP/DB_PASSWORD", wantErr: true},
		{name: "leading slash", key: "/MYKEY", wantErr: true},
		{name: "trailing slash", key: "MYKEY/", wantErr: true},

		// --- newlines (break keepassxc-cli stdin protocol) ---
		{name: "LF in key", key: "MY\nKEY", wantErr: true},
		{name: "CR in key", key: "MY\rKEY", wantErr: true},
		{name: "CRLF in key", key: "MY\r\nKEY", wantErr: true},

		// --- leading whitespace (any Unicode) ---
		{name: "leading ASCII space", key: " MYKEY", wantErr: true},
		{name: "leading tab", key: "\tMYKEY", wantErr: true},
		{name: "leading non-breaking space (U+00A0)", key: "\u00A0MYKEY", wantErr: true},

		// --- trailing whitespace (any Unicode) ---
		{name: "trailing ASCII space", key: "MYKEY ", wantErr: true},
		{name: "trailing tab", key: "MYKEY\t", wantErr: true},
		{name: "trailing non-breaking space (U+00A0)", key: "MYKEY\u00A0", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := keepassxc.ValidateKey(tc.key)
			if tc.wantErr {
				assert.True(t, errors.Is(err, keepassxc.ErrInvalidKey), "expected ErrInvalidKey, got %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseListOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "single root entry",
			input: "MY_SECRET\n",
			want:  []string{"MY_SECRET"},
		},
		{
			name:  "multiple root entries",
			input: "DB_PASSWORD\nAPI_TOKEN\nSTRIPE_KEY\n",
			want:  []string{"DB_PASSWORD", "API_TOKEN", "STRIPE_KEY"},
		},
		{
			name:  "group name is skipped",
			input: "Recycle Bin/\n",
			want:  nil,
		},
		{
			name:  "space-indented sub-entry is skipped",
			input: "Secrets/\n  OldKey\n",
			want:  nil,
		},
		{
			name:  "tab-indented sub-entry is skipped",
			input: "Group/\n\tSubEntry\n",
			want:  nil,
		},
		{
			name:  "mix of root entries, groups, and sub-entries",
			input: "MY_KEY\nRecycle Bin/\n  DeletedKey\nAPI_TOKEN\nSecrets/\n  Nested\n",
			want:  []string{"MY_KEY", "API_TOKEN"},
		},
		{
			name:  "blank lines are skipped",
			input: "KEY1\n\nKEY2\n",
			want:  []string{"KEY1", "KEY2"},
		},
		{
			name:  "CRLF line endings are handled",
			input: "KEY1\r\nKEY2\r\n",
			want:  []string{"KEY1", "KEY2"},
		},
		{
			name:  "title is not trimmed — exact characters preserved",
			input: "EXACT_KEY\n",
			want:  []string{"EXACT_KEY"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keepassxc.ParseListOutput(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
