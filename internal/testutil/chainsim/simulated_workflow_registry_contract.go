package chainsim

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

type SimulatedWorkflowRegistry struct {
	Contract common.Address
}

func DeployWorkflowRegistry(t *testing.T, ethClient *seth.Client, chain *SimulatedChain, logger *zerolog.Logger) SimulatedWorkflowRegistry {
	deployedContract, err := ethClient.DeployContractFromContractStore(ethClient.NewTXOpts(), constants.WorkflowRegistryContractName)
	require.NoError(t, err, "Failed to deploy contract")

	txcConfig := client.TxClientConfig{
		TxType:       client.Regular,
		LedgerConfig: &client.LedgerConfig{LedgerEnabled: false},
		SkipPrompt:   true,
	}
	workflowRegistryClient := client.NewWorkflowRegistryV2Client(logger, ethClient, deployedContract.Address.Hex(), txcConfig)

	err = workflowRegistryClient.UpdateAllowedSigners([]common.Address{common.HexToAddress(TestAddress)}, true)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to update authorized addresses")

	err = workflowRegistryClient.SetDonLimit(constants.DefaultProductionDonFamily, 1000, 100)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to update allowed DONs")

	err = linkOwner(workflowRegistryClient)
	require.NoError(t, err, "Failed to link owner")

	return SimulatedWorkflowRegistry{
		Contract: deployedContract.Address,
	}
}

func linkOwner(wrc *client.WorkflowRegistryV2Client) error {
	validity := time.Now().UTC().Add(time.Hour * 24)
	validityTimestamp := big.NewInt(validity.Unix())

	nonce := uuid.New().String()
	data := TestAddress + "22" + nonce
	hash := sha256.Sum256([]byte(data))
	ownershipProof := hex.EncodeToString(hash[:])

	const LinkRequestType uint8 = 0

	version, err := wrc.TypeAndVersion()
	if err != nil {
		return err
	}

	messageDigest, err := testutil.PreparePayloadForSigning(
		testutil.OwnershipProofSignaturePayload{
			RequestType:              LinkRequestType,
			WorkflowOwnerAddress:     common.HexToAddress(TestAddress),
			ChainID:                  "1337",
			WorkflowRegistryContract: wrc.ContractAddress,
			Version:                  version,
			ValidityTimestamp:        validity,
			OwnershipProofHash:       common.HexToHash(ownershipProof),
		})
	if err != nil {
		return fmt.Errorf("failed to prepare payload for signing: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(TestPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// The produced signature is in the [R || S || V] format where V is 0 or 1.
	signature, err := crypto.Sign(messageDigest, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign ownership proof: %w", err)
	}

	recoveredPubKey, err := crypto.SigToPub(messageDigest, signature)
	if err != nil {
		return err
	}
	addr := crypto.PubkeyToAddress(*recoveredPubKey)
	if addr.Hex() != TestAddress {
		return fmt.Errorf("recovered address does not match expected address: %s != %s", addr.Hex(), TestAddress)
	}
	//t.Logf("Validity timestamp: %s, OwnershipProof: %s, Signature: %s, Message Digest: %s",
	//	validityTimestamp, common.HexToHash(ownershipProof), common.Bytes2Hex(signature), common.Bytes2Hex(messageDigest))

	signature[64] += 27

	// Assuming `signature` is the byte slice returned by crypto.Sign
	//r := new(big.Int).SetBytes(signature[:32])   // First 32 bytes
	//s := new(big.Int).SetBytes(signature[32:64]) // Next 32 bytes
	//v := uint8(signature[64])                    // Last byte

	//t.Logf("v: %d, r: %s, s: %s", v, r.Text(16), s.Text(16))

	_, err = wrc.LinkOwner(validityTimestamp, common.HexToHash(ownershipProof), signature)
	if err != nil {
		return err
	}
	return nil
}
