package onepassword_test

import (
	"testing"

	"github.com/javorszky/envsecrets/internal/onepassword"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePasswordOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "plain value — trailing newline stripped",
			in:   []byte("hunter2\n"),
			want: "hunter2",
		},
		{
			name: "multiline value with LF — internal newline preserved",
			in:   []byte("line1\nline2\n"),
			want: "line1\nline2",
		},
		{
			name: "multiline value with CRLF — internal CRLF preserved",
			in:   []byte("line1\r\nline2\n"),
			want: "line1\r\nline2",
		},
		{
			name: "yaml block — all internal newlines preserved",
			in:   []byte("key: value\nnested:\n  - item1\n  - item2\n"),
			want: "key: value\nnested:\n  - item1\n  - item2",
		},
		{
			name: "value with literal percent — unchanged",
			in:   []byte("100%\n"),
			want: "100%",
		},
		{
			name: "empty output",
			in:   []byte("\n"),
			want: "",
		},
		{
			name: "no trailing newline — value unchanged",
			in:   []byte("hunter2"),
			want: "hunter2",
		},
		{
			// Regression: TrimRight would strip ALL trailing newlines, destroying
			// a secret that intentionally ends with a blank line. TrimSuffix
			// removes exactly one — the newline appended by the CLI.
			name: "value ending with newline — only the CLI newline stripped",
			in:   []byte("value\n\n"),
			want: "value\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := onepassword.ParsePasswordOutput(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

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
		{
			// Regression: op CLI may emit pretty-printed JSON with spaces after
			// colons ("name": "…"). The old strings.Split approach split on
			// `"name":"` (no space) and silently returned nil, causing EnsureVault
			// to re-create the vault on every store call.
			name:  "pretty-printed JSON with spaces after colons",
			input: "[\n  {\n    \"id\": \"abc123\",\n    \"name\": \"EnvSecrets\"\n  }\n]",
			want:  []string{"EnvSecrets"},
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
		{
			// Regression: same pretty-printed JSON issue as ParseVaultNames.
			name:  "pretty-printed JSON with spaces after colons",
			input: "[\n  {\n    \"id\": \"abc123\",\n    \"title\": \"MY_KEY\"\n  }\n]",
			want:  []string{"MY_KEY"},
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
