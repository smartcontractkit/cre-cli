package test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	vaultcfgpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"
	p2ptypes "github.com/smartcontractkit/libocr/ragep2p/types"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func DeployCapabilitiesRegistry(sethClient *seth.Client, pubKeys []*ed25519.PublicKey, p2pIds []p2ptypes.PeerID) (common.Address, error) {
	deployedContract, err := sethClient.DeployContractFromContractStore(
		sethClient.NewTXOpts(),
		constants.CapabilitiesRegistryContractName,
		capabilities_registry_wrapper_v2.CapabilitiesRegistryConstructorParams{
			CanAddOneNodeDONs: true,
		},
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy CapabilitiesRegistry contract: %w", err)
	}

	registry, err := capabilities_registry_wrapper_v2.NewCapabilitiesRegistry(deployedContract.Address, sethClient.Client)
	if err != nil {
		return common.Address{}, err
	}

	_, err = sethClient.Decode(registry.AddNodeOperators(sethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeOperator{
		{
			Admin: common.HexToAddress(constants.TestAddress),
			Name:  "operator",
		},
	}))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to add node operators to CapabilitiesRegistry: %w", err)
	}

	_, err = sethClient.Decode(registry.AddCapabilities(sethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryCapability{
		{
			CapabilityId:          vault.CapabilityID,
			ConfigurationContract: common.HexToAddress("0x0"),
			Metadata:              []byte{0x01, 0x02, 0x03}, // Example metadata, should be replaced with actual metadata
		},
	}))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to add capabilities to CapabilitiesRegistry: %w", err)
	}

	_, err = sethClient.Decode(registry.AddNodes(sethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeParams{
		{
			NodeOperatorId:      1,
			Signer:              [32]byte(crypto.Keccak256([]byte(uuid.New().String()))),
			P2pId:               p2pIds[0],
			EncryptionPublicKey: [32]byte(crypto.Keccak256([]byte(uuid.New().String()))),
			CsaKey:              [32]byte(*pubKeys[0]),
			CapabilityIds:       []string{vault.CapabilityID},
		},
	}))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to add nodes to CapabilitiesRegistry: %w", err)
	}

	vaultCfgBytes, err := buildVaultCapabilityConfigBytes([]byte(*pubKeys[0]))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to build vault capability config bytes: %w", err)
	}

	_, err = sethClient.Decode(registry.AddDONs(sethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryNewDONParams{
		{
			Name:        "zone-a",
			DonFamilies: []string{constants.DefaultStagingDonFamily},
			Config:      []byte{0x01, 0x02, 0x03}, // Example config, should be replaced with actual config
			CapabilityConfigurations: []capabilities_registry_wrapper_v2.CapabilitiesRegistryCapabilityConfiguration{
				{
					CapabilityId: vault.CapabilityID,
					Config:       vaultCfgBytes,
				},
			},
			Nodes:            [][32]byte{p2pIds[0]},
			F:                0,
			IsPublic:         true,
			AcceptsWorkflows: true,
		},
	}))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to add DONs to CapabilitiesRegistry: %w", err)
	}

	_, err = sethClient.Decode(registry.SetDONFamilies(sethClient.NewTXOpts(), 1, []string{constants.DefaultStagingDonFamily}, nil))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to set DON families in CapabilitiesRegistry: %w", err)
	}

	return deployedContract.Address, nil
}

// deployTestWorkflowRegistry deploys the WorkflowRegistry contract using the provided Seth client.
func DeployTestWorkflowRegistry(t *testing.T, sethClient *seth.Client) (*workflow_registry_wrapper_v2.WorkflowRegistry, error) {
	deployedContract, err := sethClient.DeployContractFromContractStore(
		sethClient.NewTXOpts(),
		constants.WorkflowRegistryContractName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy WorkflowRegistry contract: %w", err)
	}

	registry, err := workflow_registry_wrapper_v2.NewWorkflowRegistry(deployedContract.Address, sethClient.Client)
	if err != nil {
		return nil, err
	}
	_, err = sethClient.Decode(registry.UpdateAllowedSigners(sethClient.NewTXOpts(), []common.Address{common.HexToAddress(constants.TestAddress2)}, true))
	if err != nil {
		return nil, err
	}

	_, err = sethClient.Decode(registry.SetDONLimit(sethClient.NewTXOpts(), constants.DefaultStagingDonFamily, 100, true))
	if err != nil {
		return nil, err
	}

	validity := time.Now().UTC().Add(time.Hour * 24)
	validityTimestamp := big.NewInt(validity.Unix())

	nonce := uuid.New().String()
	data := constants.TestAddress3 + "22" + nonce
	hash := sha256.Sum256([]byte(data))
	ownershipProof := hex.EncodeToString(hash[:])

	const LinkRequestType uint8 = 0

	chainId := sethClient.ChainID
	version, err := registry.TypeAndVersion(sethClient.NewCallOpts())
	if err != nil {
		return nil, err
	}

	messageDigest, err := testutil.PreparePayloadForSigning(
		testutil.OwnershipProofSignaturePayload{
			RequestType:              LinkRequestType,
			WorkflowOwnerAddress:     common.HexToAddress(constants.TestAddress3),
			ChainID:                  strconv.FormatInt(chainId, 10),
			WorkflowRegistryContract: registry.Address(),
			Version:                  version,
			ValidityTimestamp:        validity,
			OwnershipProofHash:       common.HexToHash(ownershipProof),
		})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare payload for signing: %w", err)
	}

	// The produced signature is in the [R || S || V] format where V is 0 or 1.
	signature, err := crypto.Sign(messageDigest, sethClient.PrivateKeys[1])
	if err != nil {
		return nil, fmt.Errorf("failed to sign ownership proof: %w", err)
	}

	recoveredPubKey, err := crypto.SigToPub(messageDigest, signature)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(*recoveredPubKey)
	if addr.Hex() != constants.TestAddress2 {
		return nil, fmt.Errorf("recovered address does not match expected address: %s != %s", addr.Hex(), constants.TestAddress)
	}
	//t.Logf("Validity timestamp: %s, OwnershipProof: %s, Signature: %s, Message Digest: %s",
	//	validityTimestamp, common.HexToHash(ownershipProof), common.Bytes2Hex(signature), common.Bytes2Hex(messageDigest))

	signature[64] += 27

	// Assuming `signature` is the byte slice returned by crypto.Sign
	//r := new(big.Int).SetBytes(signature[:32])   // First 32 bytes
	//s := new(big.Int).SetBytes(signature[32:64]) // Next 32 bytes
	//v := uint8(signature[64])                    // Last byte

	//t.Logf("v: %d, r: %s, s: %s", v, r.Text(16), s.Text(16))

	_, err = sethClient.Decode(registry.LinkOwner(sethClient.NewTXKeyOpts(2), validityTimestamp, common.HexToHash(ownershipProof), signature))
	if err != nil {
		return nil, err
	}

	return registry, nil
}

// Helper function to create test Ed25519 signer and keys
func CreateTestSigner() (*core.Ed25519Signer, ed25519.PublicKey, p2ptypes.PeerID) {
	// Generate a private key for signing
	csaPubKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("Failed to generate Ed25519 key pair: " + err.Error())
	}

	// Generate a separate public key for p2pId
	p2pIdKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("Failed to generate Ed25519 p2pId: " + err.Error())
	}

	// Create PeerID from ed25519 public key
	p2pId, err := p2ptypes.PeerIDFromPublicKey(p2pIdKey)
	if err != nil {
		panic("Failed to create PeerID from public key: " + err.Error())
	}

	// Create ed25519 signer from the mock node's csa private key
	signFn := func(ctx context.Context, account string, data []byte) (signed []byte, err error) {
		return ed25519.Sign(privateKey, data), nil
	}

	signer, err := core.NewEd25519Signer(hex.EncodeToString(csaPubKey), signFn)
	if err != nil {
		panic("Failed to create Ed25519Signer: " + err.Error())
	}

	return signer, csaPubKey, p2pId
}

type OwnershipProofSignaturePayload struct {
	RequestType              uint8          // should be uint8 in Solidity, 1 byte
	WorkflowOwnerAddress     common.Address // should be 20 bytes in Solidity, address type
	ChainID                  string         // should be uint256 in Solidity, chain-selectors provide it as a string
	WorkflowRegistryContract common.Address // address of the WorkflowRegistry contract, should be 20 bytes in Solidity
	Version                  string         // should be dynamic type in Solidity (string)
	ValidityTimestamp        time.Time      // should be uint256 in Solidity
	OwnershipProofHash       common.Hash    // should be bytes32 in Solidity, 32 bytes hash of the ownership proof
}

// buildVaultCapabilityConfigBytes builds the protobuf-encoded CapabilityConfig bytes
// setting DefaultConfig["VaultPublicKey"] = <pubKeyBytes>.
func buildVaultCapabilityConfigBytes(raw []byte) ([]byte, error) {
	if len(raw) != 32 {
		return nil, fmt.Errorf("VaultPublicKey must be 32 bytes, got %d", len(raw))
	}
	cfg := &vaultcfgpb.CapabilityConfig{
		DefaultConfig: &valuespb.Map{
			Fields: map[string]*valuespb.Value{
				"VaultPublicKey": {Value: &valuespb.Value_BytesValue{BytesValue: raw}},
			},
		},
	}
	return proto.Marshal(cfg)
}

func NewSethClientWithContracts(t *testing.T, logger *zerolog.Logger, rpcUrl string, chainId uint64, configFile string) *seth.Client {
	privateKeys := []string{constants.TestPrivateKey, constants.TestPrivateKey2, constants.TestPrivateKey3}

	ethClient, err := client.NewSethClient(configFile, rpcUrl, privateKeys, chainId)
	require.NoError(t, err, "Failed to create client")

	if err := client.LoadContracts(logger, ethClient); err != nil {
		logger.Error().Err(err).Msg("Failed to load contracts")
		return nil
	}

	return ethClient
}
