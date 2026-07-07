package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "masks last path segment",
			raw:  "https://rpc.example.com/v1/my-secret-key",
			want: "https://rpc.example.com/v1/***",
		},
		{
			name: "removes query params",
			raw:  "https://rpc.example.com/v1/key?token=secret",
			want: "https://rpc.example.com/v1/***",
		},
		{
			name: "single path segment masked",
			raw:  "https://rpc.example.com/key",
			want: "https://rpc.example.com/***",
		},
		{
			name: "no path",
			raw:  "https://rpc.example.com",
			want: "https://rpc.example.com",
		},
		{
			name: "invalid URL",
			raw:  "://bad",
			want: RedactedValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, URL(tt.raw))
		})
	}
}

func TestFlag(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		value    string
		want     string
	}{
		{name: "env path", flagName: "env", value: "/home/user/project/.env", want: RedactedValue},
		{name: "public env path", flagName: "public-env", value: "/tmp/.env.public", want: RedactedValue},
		{name: "http payload", flagName: "http-payload", value: `{"secret":"value"}`, want: RedactedValue},
		{name: "public key flag", flagName: "public_key", value: "0xabc", want: RedactedValue},
		{name: "config path", flagName: "config", value: "./config.yaml", want: RedactedValue},
		{name: "limits default", flagName: "limits", value: "default", want: "default"},
		{name: "limits none", flagName: "limits", value: "none", want: "none"},
		{name: "limits file path", flagName: "limits", value: "./limits.json", want: RedactedValue},
		{name: "rpc url with secret", flagName: "rpc-url", value: "ethereum=https://rpc.example.com/v1/secret-key", want: "ethereum=https://rpc.example.com/v1/***"},
		{name: "wasm local path", flagName: "wasm", value: "./binary.wasm", want: "./binary.wasm"},
		{name: "wasm remote url", flagName: "wasm", value: "https://cdn.example.com/wasm/secret", want: "https://cdn.example.com/wasm/***"},
		{name: "benign flag", flagName: "verbose", value: "true", want: "true"},
		{name: "empty value", flagName: "env", value: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Flag(tt.flagName, tt.value))
		})
	}
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		subcommand string
		args       []string
		want       []string
	}{
		{
			name:       "secrets basename",
			action:     "secrets",
			subcommand: "create",
			args:       []string{"/home/user/project/secrets.yaml"},
			want:       []string{"secrets.yaml"},
		},
		{
			name:       "templates add benign repo",
			action:     "templates",
			subcommand: "add",
			args:       []string{"smartcontractkit/cre-templates"},
			want:       []string{"smartcontractkit/cre-templates"},
		},
		{
			name:       "templates add url with token",
			action:     "templates",
			subcommand: "add",
			args:       []string{"https://github.com/org/repo?token=secret"},
			want:       []string{RedactedValue},
		},
		{
			name:       "workflow get unchanged",
			action:     "workflow",
			subcommand: "get",
			args:       []string{"my-workflow"},
			want:       []string{"my-workflow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Args(tt.action, tt.subcommand, tt.args))
		})
	}
}

func TestErrorMessage(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"

	tests := []struct {
		name string
		msg  string
		want string
	}{
		{
			name: "jwt segment",
			msg:  "auth failed: token " + jwt,
			want: "auth failed: token " + RedactedValue,
		},
		{
			name: "bearer token",
			msg:  "request failed with Authorization: Bearer super-secret-token",
			want: "request failed with Authorization: Bearer " + RedactedValue,
		},
		{
			name: "api key header",
			msg:  "Authorization: Apikey abc123",
			want: "Authorization: Apikey " + RedactedValue,
		},
		{
			name: "private key hex",
			msg:  "invalid key ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			want: "invalid key " + RedactedValue,
		},
		{
			name: "rpc url in error",
			msg:  `dial failed: https://rpc.example.com/v1/secret-key?token=abc`,
			want: `dial failed: https://rpc.example.com/v1/***`,
		},
		{
			name: "env api key",
			msg:  "missing CRE_API_KEY=super-secret",
			want: "missing CRE_API_KEY=" + RedactedValue,
		},
		{
			name: "benign error unchanged",
			msg:  "workflow not found",
			want: "workflow not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ErrorMessage(tt.msg))
		})
	}
}

func TestSafeJWTClaimsForLog(t *testing.T) {
	claims := map[string]interface{}{
		"sub":                              "user123",
		"org_id":                           "org456",
		"exp":                              float64(1234567890),
		"https://api.cre.chain.link/email": "test@example.com",
		"https://api.cre.chain.link/organization_status": "FULL_ACCESS",
		"https://api.cre.chain.link/organization_roles":  "ROOT",
	}

	safe := SafeJWTClaimsForLog(claims)

	assert.Equal(t, "user123", safe["sub"])
	assert.Equal(t, "org456", safe["org_id"])
	assert.Equal(t, float64(1234567890), safe["exp"])
	assert.Equal(t, "FULL_ACCESS", safe["organization_status"])
	assert.Equal(t, "ROOT", safe["organization_roles"])
	assert.NotContains(t, safe, "email")
	assert.NotContains(t, safe, "https://api.cre.chain.link/email")
}
