package keychain_test

import (
	"testing"

	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/stretchr/testify/assert"
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

			got := keychain.ParsePasswordOutput(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseDumpServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "single service entry",
			input: `keychain: "/path/to/file.keychain"
version: 512
class: "genp"
    0x00000007 <blob>="myservice"
    "svce"<blob>="myservice"
    "acct"<blob>="username"
`,
			want: []string{"myservice"},
		},
		{
			name: "multiple service entries",
			input: `keychain: "/path/to/file.keychain"
version: 512
class: "genp"
    0x00000007 <blob>="DB_PASSWORD"
    "svce"<blob>="DB_PASSWORD"
    "acct"<blob>="gabor"
class: "genp"
    0x00000007 <blob>="API_KEY"
    "svce"<blob>="API_KEY"
    "acct"<blob>="gabor"
class: "genp"
    0x00000007 <blob>="STRIPE_SECRET"
    "svce"<blob>="STRIPE_SECRET"
    "acct"<blob>="gabor"
`,
			want: []string{"DB_PASSWORD", "API_KEY", "STRIPE_SECRET"},
		},
		{
			name: "duplicate service names are deduplicated",
			input: `    "svce"<blob>="SAME_KEY"
    "svce"<blob>="SAME_KEY"
`,
			want: []string{"SAME_KEY"},
		},
		{
			name: "other attributes are ignored",
			input: `keychain: "/path/to/file.keychain"
version: 512
class: "genp"
    "acct"<blob>="username"
    "cdat"<timedate>=0x32303234303431365431323030303057  "20240416T120000Z\000"
    "mdat"<timedate>=0x32303234303431365431323030303057  "20240416T120000Z\000"
    "pdmn"<sint32>=0x00000007
`,
			want: nil,
		},
		{
			name: "service names with special characters",
			input: `    "svce"<blob>="my-app_DB_PASSWORD"
    "svce"<blob>="project.api.key"
    "svce"<blob>="secret/with/slashes"
`,
			want: []string{"my-app_DB_PASSWORD", "project.api.key", "secret/with/slashes"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keychain.ParseDumpServices(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}
