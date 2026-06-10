package vaultdon

import (
	"context"
	"fmt"
	"math/big"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/internal/onchain/capabilitiesregistry"
)

// Reader loads vault DON metadata from a CapabilitiesRegistry contract.
type Reader interface {
	GetDONsInFamily(ctx context.Context, family string) ([]*big.Int, error)
	GetDON(ctx context.Context, donID uint32) (capreg.CapabilitiesRegistryDONInfo, error)
	GetNodes(ctx context.Context) ([]capreg.INodeInfoProviderNodeInfo, error)
}

// VaultDON is the vault capability DON and its member nodes from the registry.
type VaultDON struct {
	DON   capreg.CapabilitiesRegistryDONInfo
	Nodes []capreg.INodeInfoProviderNodeInfo
}

// Resolver resolves the tenant vault DON for a DON family via CapabilitiesRegistry reads.
type Resolver struct {
	reader Reader
	family string

	cached *VaultDON
}

// NewResolver constructs a vault DON resolver for the given family.
func NewResolver(reader Reader, family string) *Resolver {
	return &Resolver{reader: reader, family: family}
}

// ResolveVaultDON finds the unique vault@1.0.0 DON in the configured family and loads its nodes.
func (r *Resolver) ResolveVaultDON(ctx context.Context) (*VaultDON, error) {
	if r.cached != nil {
		return r.cached, nil
	}

	donIDs, err := r.reader.GetDONsInFamily(ctx, r.family)
	if err != nil {
		return nil, err
	}

	var vaultDONs []capreg.CapabilitiesRegistryDONInfo
	for _, id := range donIDs {
		if id == nil || !id.IsUint64() {
			continue
		}
		don, err := r.reader.GetDON(ctx, uint32(id.Uint64())) // #nosec G115 -- guarded by IsUint64
		if err != nil {
			return nil, err
		}
		if donHostsVaultCapability(&don) {
			vaultDONs = append(vaultDONs, don)
		}
	}

	if len(vaultDONs) == 0 {
		return nil, fmt.Errorf("no vault DON found in family %q for capability %s", r.family, vaultcommon.CapabilityID)
	}
	if len(vaultDONs) > 1 {
		return nil, fmt.Errorf("multiple vault DONs (%d) found in family %q; expected exactly one hosting %s",
			len(vaultDONs), r.family, vaultcommon.CapabilityID)
	}

	allNodes, err := r.reader.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := nodesForDON(&vaultDONs[0], allNodes)
	if err != nil {
		return nil, err
	}

	r.cached = &VaultDON{DON: vaultDONs[0], Nodes: nodes}
	return r.cached, nil
}

// VaultPublicKeyHex returns the on-chain vault master public key for the resolved DON.
func VaultPublicKeyHex(v *VaultDON) (string, error) {
	if v == nil {
		return "", fmt.Errorf("vault DON is nil")
	}
	raw, err := vaultCapabilityConfigBytes(v)
	if err != nil {
		return "", err
	}
	cfg, err := parseVaultCapabilityConfig(raw)
	if err != nil {
		return "", err
	}
	return cfg.VaultPublicKey, nil
}

func donHostsVaultCapability(don *capreg.CapabilitiesRegistryDONInfo) bool {
	for _, cc := range don.CapabilityConfigurations {
		if cc.CapabilityId == vaultcommon.CapabilityID {
			return true
		}
	}
	return false
}

type vaultCapabilityRegistryConfig struct {
	VaultPublicKey string `mapstructure:"VaultPublicKey"`
	Threshold      int    `mapstructure:"Threshold"`
}

func parseVaultCapabilityConfig(raw []byte) (*vaultCapabilityRegistryConfig, error) {
	cfg, err := capabilitiesregistry.ParseCapabilityConfiguration(raw)
	if err != nil {
		return nil, err
	}

	out := &vaultCapabilityRegistryConfig{}
	if err := cfg.DefaultConfig.UnwrapTo(out); err != nil {
		return nil, fmt.Errorf("unwrap vault capability config: %w", err)
	}
	if out.VaultPublicKey == "" {
		return nil, fmt.Errorf("VaultPublicKey is not provided in the capability config")
	}
	if out.Threshold <= 0 {
		return nil, fmt.Errorf("invalid Threshold in the capability config")
	}
	return out, nil
}

func vaultCapabilityConfigBytes(v *VaultDON) ([]byte, error) {
	for _, cc := range v.DON.CapabilityConfigurations {
		if cc.CapabilityId == vaultcommon.CapabilityID {
			if len(cc.Config) == 0 {
				return nil, fmt.Errorf("vault capability config is empty on DON %d", v.DON.Id)
			}
			return cc.Config, nil
		}
	}
	return nil, fmt.Errorf("vault capability %s not found on DON %d", vaultcommon.CapabilityID, v.DON.Id)
}

func nodesForDON(don *capreg.CapabilitiesRegistryDONInfo, allNodes []capreg.INodeInfoProviderNodeInfo) ([]capreg.INodeInfoProviderNodeInfo, error) {
	byP2P := make(map[[32]byte]capreg.INodeInfoProviderNodeInfo, len(allNodes))
	for _, node := range allNodes {
		byP2P[node.P2pId] = node
	}

	nodes := make([]capreg.INodeInfoProviderNodeInfo, 0, len(don.NodeP2PIds))
	for _, p2pID := range don.NodeP2PIds {
		node, ok := byP2P[p2pID]
		if !ok {
			return nil, fmt.Errorf("DON %d references node p2p id %x not found in registry", don.Id, p2pID)
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("DON %d has no member nodes", don.Id)
	}
	return nodes, nil
}
