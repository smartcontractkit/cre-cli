package common

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/gateway"
	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func (h *Handler) vaultKeyPinScope() (tenantctx.VaultKeyPinScope, error) {
	if h.TenantContext == nil {
		return tenantctx.VaultKeyPinScope{}, fmt.Errorf("tenant context is not available; run `cre login` to refresh")
	}
	if h.TenantContext.CapabilitiesRegistry == nil {
		return tenantctx.VaultKeyPinScope{}, fmt.Errorf("capabilities registry is not configured in your user context; run `cre login` to refresh")
	}
	if h.EnvironmentSet == nil {
		return tenantctx.VaultKeyPinScope{}, fmt.Errorf("environment is not configured")
	}

	envName := strings.TrimSpace(h.EnvironmentSet.EnvName)
	if envName == "" {
		envName = "production"
	}

	return tenantctx.VaultKeyPinScope{
		EnvName:             envName,
		TenantID:            h.TenantContext.TenantID,
		CapRegChainSelector: h.TenantContext.CapabilitiesRegistry.ChainSelector,
		CapRegAddress:       h.TenantContext.CapabilitiesRegistry.Address,
		VaultGatewayURL:     gateway.ResolveVaultGatewayURL(h.TenantContext, h.EnvironmentSet),
	}, nil
}

func (h *Handler) verifyVaultKeyTOFU(gatewayFP, onChainFP string) error {
	scope, err := h.vaultKeyPinScope()
	if err != nil {
		return err
	}

	pinnedFP, ok, err := tenantctx.LoadVaultKeyPin(scope)
	if err != nil {
		return fmt.Errorf("load vault public key pin: %w", err)
	}
	if !ok || tenantctx.FingerprintsMatch(pinnedFP, gatewayFP) {
		return nil
	}
	if tenantctx.FingerprintsMatch(pinnedFP, onChainFP) {
		return fmt.Errorf(
			"vault public key from gateway changed without a matching on-chain update; remove %s to re-trust after verifying the gateway",
			creconfig.FilePathHint(tenantctx.VaultKeyPinsFile),
		)
	}
	return nil
}

func (h *Handler) persistVaultKeyPin(gatewayFP string) error {
	scope, err := h.vaultKeyPinScope()
	if err != nil {
		return err
	}
	if err := tenantctx.SaveVaultKeyPin(scope, gatewayFP); err != nil {
		return fmt.Errorf("persist vault public key pin: %w", err)
	}
	return nil
}
