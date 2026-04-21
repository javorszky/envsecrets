package keepassxc_test

import (
	"testing"

	"github.com/javorszky/envsecrets/internal/keepassxc"
	"github.com/stretchr/testify/assert"
)

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
