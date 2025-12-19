package graphqlclient

import (
	"testing"
)

func TestRedactSensitiveHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "redacts bearer token",
			input:    ">> headers: map[Authorization:[Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.longtoken.signature] Content-Type:[application/json]]",
			expected: ">> headers: map[Authorization:[[REDACTED]] Content-Type:[application/json]]",
		},
		{
			name:     "redacts api key",
			input:    ">> headers: map[Authorization:[Apikey sk_live_abc123xyz789] User-Agent:[cre-cli]]",
			expected: ">> headers: map[Authorization:[[REDACTED]] User-Agent:[cre-cli]]",
		},
		{
			name:     "no change for messages without authorization",
			input:    ">> query: mutation { createUser }",
			expected: ">> query: mutation { createUser }",
		},
		{
			name:     "no change for response messages",
			input:    "<< {\"data\":{\"user\":{\"id\":\"123\"}}}",
			expected: "<< {\"data\":{\"user\":{\"id\":\"123\"}}}",
		},
		{
			name:     "handles variables message",
			input:    ">> variables: map[email:test@example.com]",
			expected: ">> variables: map[email:test@example.com]",
		},
		{
			name:     "redacts short token",
			input:    ">> headers: map[Authorization:[Bearer abc]]",
			expected: ">> headers: map[Authorization:[[REDACTED]]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveHeaders(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveHeaders(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
