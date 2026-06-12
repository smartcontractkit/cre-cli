package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/vaultdon"
	"github.com/smartcontractkit/cre-cli/internal/onchain/capabilitiesregistry"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const vaultValidationSkippedWarning = "Vault gateway validation skipped; the encryption key and response signatures will not be verified independently of the gateway."

// vaultValidationGateEnabled toggles CapabilitiesRegistry RPC resolution and consent
// before secrets commands.
const vaultValidationGateEnabled = false

// EnsureVaultValidationOrConsent resolves CapabilitiesRegistry RPC settings and either
// enables on-chain validation (skipValidation=false) or obtains explicit consent to
// proceed without validation. The result is cached for the lifetime of the Handler so
// encrypt and response parsing in the same command only prompt once.
func (h *Handler) EnsureVaultValidationOrConsent(ctx context.Context) (skipValidation bool, err error) {
	if h.vaultValidationDecided {
		return h.skipVaultValidation, nil
	}

	if !vaultValidationGateEnabled {
		h.skipVaultValidation = true
		h.vaultValidationDecided = true
		return true, nil
	}

	rpcURL, chainName, ok, err := settings.ResolveCapabilitiesRegistryRPC(h.Viper, h.TenantContext)
	if err != nil {
		return false, err
	}

	if ok {
		if err := h.initVaultDONResolver(ctx, rpcURL); err != nil {
			return false, err
		}
		h.capRegRPCURL = rpcURL
		h.capRegChainName = chainName
		h.skipVaultValidation = false
		h.vaultValidationDecided = true
		return false, nil
	}

	if h.Viper.GetBool(settings.Flags.NonInteractive.Name) && !h.Viper.GetBool(settings.Flags.SkipConfirmation.Name) {
		ui.ErrorWithSuggestions(
			fmt.Sprintf("Vault gateway validation requires an RPC for %s in your project settings", chainName),
			[]string{"--yes"},
		)
		return false, fmt.Errorf("missing RPC for capabilities registry chain %q", chainName)
	}

	if h.Viper.GetBool(settings.Flags.SkipConfirmation.Name) {
		ui.Warning(vaultValidationSkippedWarning)
		h.capRegChainName = chainName
		h.skipVaultValidation = true
		h.vaultValidationDecided = true
		return true, nil
	}

	prompt := fmt.Sprintf(
		"Vault gateway responses cannot be validated without an RPC for %s in your project settings. Proceeding without validation means the CLI cannot verify the encryption key or DON signatures independently of the gateway. Proceed anyway?",
		chainName,
	)
	proceed, err := ui.Confirm(prompt)
	if err != nil {
		return false, err
	}
	if !proceed {
		return false, fmt.Errorf("aborted: vault gateway validation requires an RPC for %q", chainName)
	}

	ui.Warning(vaultValidationSkippedWarning)
	h.capRegChainName = chainName
	h.skipVaultValidation = true
	h.vaultValidationDecided = true
	return true, nil
}

// SkipVaultValidation reports whether the current command opted out of on-chain validation.
func (h *Handler) SkipVaultValidation() bool {
	return h.skipVaultValidation
}

// CapabilitiesRegistryRPC returns the validated RPC URL when validation is enabled.
func (h *Handler) CapabilitiesRegistryRPC() (rpcURL string, ok bool) {
	if h.skipVaultValidation || h.capRegRPCURL == "" {
		return "", false
	}
	return h.capRegRPCURL, true
}

// CapabilitiesRegistryChainName returns the chain name for the tenant CapabilitiesRegistry.
func (h *Handler) CapabilitiesRegistryChainName() string {
	return h.capRegChainName
}

// VaultDONResolver returns the shared vault DON resolver when on-chain validation is enabled.
func (h *Handler) VaultDONResolver() (*vaultdon.Resolver, bool) {
	if h.skipVaultValidation || h.vaultDONResolver == nil {
		return nil, false
	}
	return h.vaultDONResolver, true
}

func (h *Handler) initVaultDONResolver(ctx context.Context, rpcURL string) error {
	if h.TenantContext == nil || h.TenantContext.CapabilitiesRegistry == nil {
		return fmt.Errorf("capabilities registry is not configured in your user context; run `cre login` to refresh")
	}

	family := settings.EffectiveDonFamily(h.EnvironmentSet, h.TenantContext)
	if family == "" {
		return fmt.Errorf("don family is not configured; run `cre login` to refresh")
	}

	client, err := capabilitiesregistry.NewReadOnlyClient(
		ctx,
		rpcURL,
		h.TenantContext.CapabilitiesRegistry.Address,
	)
	if err != nil {
		return fmt.Errorf("failed to create capabilities registry client: %w", err)
	}

	h.capRegClient = client
	h.vaultDONResolver = vaultdon.NewResolver(client, strings.TrimSpace(family))
	return nil
}
