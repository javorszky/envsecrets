package onepassword_test

import (
	"testing"

	"github.com/javorszky/envsecrets/internal/onepassword"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVaultNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty array",
			input: `[]`,
			want:  nil,
		},
		{
			name:  "single vault",
			input: `[{"id":"abc123","name":"Private","type":"U","content_version":196}]`,
			want:  []string{"Private"},
		},
		{
			name: "multiple vaults",
			input: `[` +
				`{"id":"aaa","name":"Private","type":"U"},` +
				`{"id":"bbb","name":"Work","type":"U"},` +
				`{"id":"ccc","name":"envsecrets","type":"U"}` +
				`]`,
			want: []string{"Private", "Work", "envsecrets"},
		},
		{
			name:  "vault name with spaces and mixed case",
			input: `[{"id":"abc","name":"My Secrets","type":"U"}]`,
			want:  []string{"My Secrets"},
		},
		{
			name:  "empty string input",
			input: ``,
			want:  nil,
		},
		{
			name:  "no name fields present",
			input: `[{"id":"abc","title":"something"}]`,
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := onepassword.ParseVaultNames(tc.input)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseTitles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty array",
			input: `[]`,
			want:  nil,
		},
		{
			name:  "single item",
			input: `[{"id":"abc123","title":"MY_KEY","vault":{"id":"xyz"}}]`,
			want:  []string{"MY_KEY"},
		},
		{
			name: "multiple items",
			input: `[` +
				`{"id":"aaa","title":"KEY_ONE","vault":{"id":"v1"}},` +
				`{"id":"bbb","title":"KEY_TWO","vault":{"id":"v1"}},` +
				`{"id":"ccc","title":"KEY_THREE","vault":{"id":"v1"}}` +
				`]`,
			want: []string{"KEY_ONE", "KEY_TWO", "KEY_THREE"},
		},
		{
			name:  "title with underscores and uppercase",
			input: `[{"id":"abc","title":"MY_APP_DB_PASSWORD","vault":{}}]`,
			want:  []string{"MY_APP_DB_PASSWORD"},
		},
		{
			name:  "empty string input — no titles",
			input: ``,
			want:  nil,
		},
		{
			name:  "no title fields present",
			input: `[{"id":"abc","name":"something"}]`,
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := onepassword.ParseTitles(tc.input)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
