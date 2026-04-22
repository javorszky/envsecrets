package keepassxc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		encoded string
	}{
		{
			name:    "plain value unchanged",
			raw:     "hunter2",
			encoded: "hunter2",
		},
		{
			name:    "value with LF newline",
			raw:     "line1\nline2",
			encoded: "line1%0Aline2",
		},
		{
			name:    "value with CRLF",
			raw:     "line1\r\nline2",
			encoded: "line1%0D%0Aline2",
		},
		{
			name:    "value with literal percent",
			raw:     "100%",
			encoded: "100%25",
		},
		{
			name:    "value with literal percent-zero-A (must not decode to newline)",
			raw:     "100%0A",
			encoded: "100%250A",
		},
		{
			name:    "yaml block with multiple newlines",
			raw:     "key: value\nnested:\n  - item1\n  - item2",
			encoded: "key: value%0Anested:%0A  - item1%0A  - item2",
		},
		{
			name:    "empty string",
			raw:     "",
			encoded: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.encoded, encodeValue(tc.raw), "encodeValue")
			assert.Equal(t, tc.raw, decodeValue(tc.encoded), "decodeValue")
			// Round-trip: encode then decode returns the original.
			assert.Equal(t, tc.raw, decodeValue(encodeValue(tc.raw)), "round-trip")
		})
	}
}
