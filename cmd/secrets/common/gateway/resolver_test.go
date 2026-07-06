package gateway

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func TestResolveVaultGatewayURL(t *testing.T) {
	embeddedURL := "https://embedded.example.com/"
	contextURL := "https://context.example.com/"
	envOverrideURL := "https://env-override.example.com/"

	envSet := &environments.EnvironmentSet{GatewayURL: embeddedURL}
	tenantCtx := &tenantctx.EnvironmentContext{VaultGatewayURL: contextURL}

	t.Run("env var wins over context URL", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, envOverrideURL)
		envSetWithOverride := &environments.EnvironmentSet{GatewayURL: envOverrideURL}
		got := ResolveVaultGatewayURL(tenantCtx, envSetWithOverride)
		if got != envOverrideURL {
			t.Errorf("got %q, want %q", got, envOverrideURL)
		}
	})

	t.Run("context URL when env var unset", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, "")
		got := ResolveVaultGatewayURL(tenantCtx, envSet)
		if got != contextURL {
			t.Errorf("got %q, want %q", got, contextURL)
		}
	})

	t.Run("embedded default when env var unset and context URL empty", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, "")
		got := ResolveVaultGatewayURL(&tenantctx.EnvironmentContext{}, envSet)
		if got != embeddedURL {
			t.Errorf("got %q, want %q", got, embeddedURL)
		}
	})

	t.Run("embedded default when tenant context nil", func(t *testing.T) {
		t.Setenv(environments.EnvVarVaultGatewayURL, "")
		got := ResolveVaultGatewayURL(nil, envSet)
		if got != embeddedURL {
			t.Errorf("got %q, want %q", got, embeddedURL)
		}
	})
}
