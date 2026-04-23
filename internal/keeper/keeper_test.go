package keeper

import (
	"testing"
)

func TestValidateKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "valid simple", key: "MY_SECRET", wantErr: false},
		{name: "valid with dots", key: "db.password", wantErr: false},
		{name: "valid with hyphens", key: "api-key-prod", wantErr: false},
		{name: "empty", key: "", wantErr: true},
		{name: "leading space", key: " KEY", wantErr: true},
		{name: "trailing space", key: "KEY ", wantErr: true},
		{name: "leading tab", key: "\tKEY", wantErr: true},
		{name: "trailing newline", key: "KEY\n", wantErr: true},
		{name: "embedded newline", key: "KE\nY", wantErr: true},
		{name: "embedded carriage return", key: "KE\rY", wantErr: true},
		{name: "embedded tab", key: "KE\tY", wantErr: true},
		{name: "embedded null byte", key: "KE\x00Y", wantErr: true},
		{name: "only whitespace", key: "   ", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateKey(%q) error = %v, wantErr = %v", tc.key, err, tc.wantErr)
			}
		})
	}
}

func TestValidateOAT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		oat     string
		wantErr bool
	}{
		{name: "valid legacy (no colon)", oat: "BASE64TOKENVALUE", wantErr: false},
		{name: "valid region prefix", oat: "US:BASE64TOKENVALUE", wantErr: false},
		{name: "valid EU region", oat: "EU:BASE64TOKENVALUE", wantErr: false},
		{name: "valid full hostname", oat: "keepersecurity.com:BASE64TOKENVALUE", wantErr: false},
		{name: "empty host part", oat: ":BASE64TOKENVALUE", wantErr: true},
		{name: "empty key part", oat: "US:", wantErr: true},
		{name: "both parts empty", oat: ":", wantErr: true},
		{name: "three parts", oat: "US:abc:def", wantErr: true},
		{name: "four parts", oat: "US:abc:def:ghi", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateOAT(tc.oat)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateOAT(%q) error = %v, wantErr = %v", tc.oat, err, tc.wantErr)
			}
		})
	}
}
