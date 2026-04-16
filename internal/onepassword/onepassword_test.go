package onepassword

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

			got, err := parseTitles(tc.input)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
