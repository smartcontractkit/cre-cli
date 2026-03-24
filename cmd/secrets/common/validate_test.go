package common

import (
	"testing"
)

func TestValidateSecretsAuthFlow(t *testing.T) {
	tests := []struct {
		name    string
		flow    string
		env     string
		wantErr bool
		errMsg  string
	}{
		{"owner-key-signing in production", SecretsAuthOwnerKeySigning, "PRODUCTION", false, ""},
		{"owner-key-signing in staging", SecretsAuthOwnerKeySigning, "STAGING", false, ""},
		{"owner-key-signing in dev", SecretsAuthOwnerKeySigning, "DEVELOPMENT", false, ""},
		{"owner-key-signing empty env defaults safe", SecretsAuthOwnerKeySigning, "", false, ""},
		{"browser in staging", SecretsAuthBrowser, "STAGING", false, ""},
		{"browser in dev", SecretsAuthBrowser, "DEVELOPMENT", false, ""},
		{"browser in production blocked", SecretsAuthBrowser, "PRODUCTION", true, "not yet available in production"},
		{"browser in production lowercase", SecretsAuthBrowser, "production", true, "not yet available in production"},
		{"browser empty env treated as production", SecretsAuthBrowser, "", true, "not yet available in production"},
		{"unknown value rejected", "magic", "STAGING", true, "unknown --secrets-auth value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsAuthFlow(tt.flow, tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsBrowserFlow(t *testing.T) {
	if IsBrowserFlow(SecretsAuthOwnerKeySigning) {
		t.Error("owner-key-signing should not be browser flow")
	}
	if !IsBrowserFlow(SecretsAuthBrowser) {
		t.Error("browser should be browser flow")
	}
	if IsBrowserFlow("unknown") {
		t.Error("unknown should not be browser flow")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
