package client

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capv2 "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

// ICapabilitiesRegistry is an interface that defines the methods we need to call
// on the generated capabilities registry contract.
type ICapabilitiesRegistry interface {
	GetDONsInFamily(opts *bind.CallOpts, donFamily string) ([]*big.Int, error)
	GetDON(opts *bind.CallOpts, donId uint32) (capv2.CapabilitiesRegistryDONInfo, error)
}

// CapabilitiesRegistryClient is the client struct.
type CapabilitiesRegistryClient struct {
	TxClient
	ContractAddress common.Address
	Cr              ICapabilitiesRegistry // Use the interface for the contract client
}

// NewCapabilitiesRegistryClient creates a new client and binds to the real contract.
func NewCapabilitiesRegistryClient(logger *zerolog.Logger, ethClient *seth.Client, contract common.Address) *CapabilitiesRegistryClient {
	// Create the real capabilities registry client
	cr, err := capv2.NewCapabilitiesRegistry(contract, ethClient.Client)
	if err != nil {
		logger.Error().Err(err).Msg("Error binding to capabilities registry contract")
		return nil
	}
	return &CapabilitiesRegistryClient{
		TxClient: TxClient{
			Logger:    logger,
			EthClient: ethClient,
		},
		ContractAddress: contract,
		Cr:              cr,
	}
}

// GetVaultMasterPublicKey finds a DON by family, then searches for a specific
// capability and returns the vault master public key in bytes.
func (wrapper *CapabilitiesRegistryClient) GetVaultMasterPublicKey(donFamily string) ([]byte, error) {
	log := wrapper.Logger

	capabilitiesRegistry := wrapper.ContractAddress.String()
	log.Debug().
		Str("Capabilities Registry Address", capabilitiesRegistry).
		Str("DON Family", donFamily).
		Msg("Starting to fetch Vault DON master public key to be able to encrypt secrets (ensure you are connected to a stable RPC provider)")

	donIds, err := wrapper.Cr.GetDONsInFamily(nil, donFamily)
	if err != nil {
		return nil, fmt.Errorf("error getting DONs in family %s: %w", donFamily, err)
	}

	if len(donIds) == 0 {
		return nil, fmt.Errorf("no DONs found for the provided donFamily: %s", donFamily)
	}

	for _, donIDBigInt := range donIds {
		// nolint:gosec
		donID := uint32(donIDBigInt.Uint64())
		donInfo, err := wrapper.Cr.GetDON(nil, donID)
		if err != nil {
			log.Error().
				Uint32("donID", donID).
				Err(err).
				Msg("Failed to get DON info")
			continue // Skip to the next DON if there's an error
		}

		for _, capConfig := range donInfo.CapabilityConfigurations {
			if capConfig.CapabilityId == vault.CapabilityID {
				log.Info().
					Str("capability", "vault.CapabilityID").
					Uint32("donID", donID).
					Hex("rawConfigBytes", capConfig.Config).
					Msg("Found a DON with the required capability")

				// Decode the Protobuf bytes.
				var configFromProto pb.CapabilityConfig
				err := proto.Unmarshal(capConfig.Config, &configFromProto)
				if err != nil {
					log.Fatal().Err(err).Msg("Failed to unmarshal protobuf config")

				}

				// Extract the public key from the decoded config.
				publicKey, err := extractVaultPublicKeyFromCapabilityConfig(&configFromProto)
				if err != nil {
					log.Fatal().Err(err).Msg("Failed to extract public key from config")
				}

				return publicKey, nil
			}
		}
	}

	return nil, fmt.Errorf("no DON found with the required '%s' capability", vault.CapabilityID)
}

func extractVaultPublicKeyFromCapabilityConfig(config *pb.CapabilityConfig) ([]byte, error) {
	if config.DefaultConfig == nil {
		return []byte{}, fmt.Errorf("DefaultConfig is nil")
	}

	vaultPublicKey, ok := config.DefaultConfig.Fields["VaultPublicKey"]
	if !ok {
		return []byte{}, fmt.Errorf("VaultPublicKey is not provided in the capability config")
	}
	return vaultPublicKey.GetBytesValue(), nil
}
