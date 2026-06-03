package settings

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/rpc"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// ResolveCapabilitiesRegistryRPC looks up the project RPC URL for the tenant's
// CapabilitiesRegistry chain. When no RPC is configured, ok is false and err is nil.
// When an RPC is configured, its URL format and eth_chainId are validated against
// the tenant chain selector before returning ok=true.
//
// TODO(DEVSVCS-5178)
func ResolveCapabilitiesRegistryRPC(v *viper.Viper, tenantCtx *tenantctx.EnvironmentContext) (rpcURL, chainName string, ok bool, err error) {
	if tenantCtx == nil || tenantCtx.CapabilitiesRegistry == nil {
		return "", "", false, fmt.Errorf("capabilities registry is not configured in your user context; run `cre login` to refresh %s", tenantctx.ContextFile)
	}

	expectedSelector := tenantCtx.CapabilitiesRegistry.ChainSelector

	chainName, err = GetChainNameByChainSelector(expectedSelector)
	if err != nil {
		return "", "", false, fmt.Errorf("capabilities registry chain selector %d: %w", expectedSelector, err)
	}

	rpcURL, err = GetRpcUrlSettings(v, chainName)
	if err != nil {
		if strings.Contains(err.Error(), "rpc url not found") {
			return "", chainName, false, nil
		}
		return "", chainName, false, err
	}

	if err := rpc.IsValidURL(rpcURL); err != nil {
		return "", chainName, false, fmt.Errorf("invalid RPC URL for %s: %w", chainName, err)
	}

	if err := rpc.ValidateMatchesSelector(rpcURL, expectedSelector); err != nil {
		return "", chainName, false, err
	}

	return rpcURL, chainName, true, nil
}
