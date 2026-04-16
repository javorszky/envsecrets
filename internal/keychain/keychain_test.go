package keychain_test

import (
	"testing"

	"github.com/javorszky/envsecrets/internal/keychain"
	"github.com/stretchr/testify/assert"
)

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
