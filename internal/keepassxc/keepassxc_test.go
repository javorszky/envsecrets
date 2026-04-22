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
		{
			name:    "plain env var name",
			key:     "DB_PASSWORD",
			wantErr: false,
		},
		{
			name:    "name with hyphens",
			key:     "my-secret-key",
			wantErr: false,
		},
		{
			name:    "empty key rejected",
			key:     "",
			wantErr: true,
		},
		{
			name:    "slash in middle rejected",
			key:     "MYAPP/DB_PASSWORD",
			wantErr: true,
		},
		{
			name:    "leading slash rejected",
			key:     "/MYKEY",
			wantErr: true,
		},
		{
			name:    "trailing slash rejected",
			key:     "MYKEY/",
			wantErr: true,
		},
		{
			name:    "leading space rejected",
			key:     " MYKEY",
			wantErr: true,
		},
		{
			name:    "leading tab rejected",
			key:     "\tMYKEY",
			wantErr: true,
		},
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
			name:  "sub-entry indented under group is skipped",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keepassxc.ParseListOutput(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
