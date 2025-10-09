package test

import (
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

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func DeployBalanceReader(sethClient *seth.Client) (common.Address, error) {
	deployedContract, err := sethClient.DeployContractFromContractStore(
		sethClient.NewTXOpts(),
		constants.BalanceReaderContractName,
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy BalanceReader contract: %w", err)
	}

	return deployedContract.Address, nil
}

func DeployWERC20Mock(sethClient *seth.Client) (common.Address, error) {
	deployedContract, err := sethClient.DeployContractFromContractStore(
		sethClient.NewTXOpts(),
		constants.WERC20MockContractName,
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy WERC20Mock contract: %w", err)
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

	_, err = sethClient.Decode(registry.SetDONLimit(sethClient.NewTXOpts(), constants.DefaultStagingDonFamily, 100, 10))
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
