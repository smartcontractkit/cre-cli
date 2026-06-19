package common

import (
	"context"
	"fmt"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/internal/onchain/capabilitiesregistry"
)

func (h *Handler) verifyVaultGatewayResponse(
	ctx context.Context,
	rpcResp *jsonrpc2.Response[vaulttypes.SignedOCRResponse],
	requestID string,
) error {
	if h.SkipVaultValidation() {
		return nil
	}
	if requestID == "" {
		return fmt.Errorf("missing request ID for vault response verification")
	}
	if rpcResp.ID != requestID {
		return fmt.Errorf("jsonrpc id mismatch: got %q want %q", rpcResp.ID, requestID)
	}

	resolver, ok := h.VaultDONResolver()
	if !ok {
		return nil
	}

	signed := rpcResp.Result
	if signed == nil {
		return fmt.Errorf("empty SignedOCRResponse result")
	}
	// TODO(DEVSVCS-5365)
	if len(signed.Signatures) == 0 {
		return nil
	}

	v, err := resolver.ResolveVaultDON(ctx)
	if err != nil {
		return fmt.Errorf("resolve vault DON for signature verification: %w", err)
	}

	signers := capabilitiesregistry.OCRSignerAddresses(v.Nodes)
	minSigs := capabilitiesregistry.MinOCRSignatures(v.DON.F)
	if err := vaulttypes.ValidateSignatures(signed, signers, minSigs); err != nil {
		return fmt.Errorf("vault response signature verification failed: %w", err)
	}
	return nil
}
