package gateway

import (
	"os"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// ResolveVaultGatewayURL returns the vault gateway URL for secrets operations.
// Precedence: CRE_VAULT_DON_GATEWAY_URL env var, then context.yaml vault_gateway_url,
// then the embedded default from EnvironmentSet.
func ResolveVaultGatewayURL(tenantCtx *tenantctx.EnvironmentContext, envSet *environments.EnvironmentSet) string {
	if os.Getenv(environments.EnvVarVaultGatewayURL) != "" && envSet != nil {
		return strings.TrimSpace(envSet.GatewayURL)
	}
	if tenantCtx != nil {
		if u := strings.TrimSpace(tenantCtx.VaultGatewayURL); u != "" {
			return u
		}
	}
	if envSet != nil {
		return strings.TrimSpace(envSet.GatewayURL)
	}
	return ""
}
