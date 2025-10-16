package client

import (
	"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

type workflowRegistryV2Contract interface {
	AllowlistRequest(opts *bind.TransactOpts, requestDigest [32]byte, expiryTimestamp uint32) (*types.Transaction, error)
	IsRequestAllowlisted(opts *bind.CallOpts, owner common.Address, requestDigest [32]byte) (bool, error)
}

type WorkflowRegistryV2Client struct {
	TxClient
	ContractAddress common.Address
	Wr              workflowRegistryV2Contract
}

type RegisterWorkflowV2Parameters struct {
	WorkflowName string   // required: user specified human readable workflow name label
	Tag          string   // required: User input tag for the workflow, e.g. "Por Workflow v2"
	WorkflowID   [32]byte // required: generated based on the workflow content and owner address
	Status       uint8    // required
	DonFamily    string   // required: DON family label

	BinaryURL  string // required: URL location for the workflow binary WASM file
	ConfigURL  string // optional: URL location for the workflow configuration file (default empty string)
	Attributes []byte // optional: 1 to pause workflow after registration, 0 to activate it (default is 0)
	KeepAlive  bool   // optional: whether to keep the other workflows of the same name and owner active after the new deploy (default is false)
}

func NewWorkflowRegistryV2Client(logger *zerolog.Logger, ethClient *seth.Client, address string, txcConfig TxClientConfig) *WorkflowRegistryV2Client {
	// Create the real workflow registry client
	wr, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(common.HexToAddress(address), ethClient.Client)
	if err != nil {
		logger.Error().Err(err).Msg("Error binding to workflow registry contract")
		return nil
	}

	contractAddr := common.HexToAddress(address)
	abi, _ := workflow_registry_v2_wrapper.WorkflowRegistryMetaData.GetAbi()
	return &WorkflowRegistryV2Client{
		TxClient:        TxClient{Logger: logger, EthClient: ethClient, abi: abi, config: txcConfig},
		ContractAddress: contractAddr,
		Wr:              wr,
	}
}

func (wrc *WorkflowRegistryV2Client) LinkOwner(validityTimestamp *big.Int, proof [32]byte, signature []byte) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.LinkOwner(opts, validityTimestamp, proof, signature)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "LinkOwner", "OwnershipLinkUpdated", validityTimestamp, proof, signature)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call LinkOwner")
		return nil, err
	}

	wrc.Logger.Debug().
		Interface("tx", txOut.RawTx).
		Msg("LinkOwner transaction submitted")

	wrc.Logger.Debug().
		Str("txHash", txOut.Hash.String()).
		Msg("Owner linked successfully")
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) UnlinkOwner(owner common.Address, validityTimestamp *big.Int, signature []byte) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.UnlinkOwner(opts, owner, validityTimestamp, signature)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "UnlinkOwner", "OwnershipLinkUpdated", owner, validityTimestamp, signature)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call UnlinkOwner")
		return nil, err
	}

	wrc.Logger.Debug().
		Interface("tx", txOut.RawTx).
		Msg("UnlinkOwner transaction submitted")

	wrc.Logger.Debug().
		Str("txHash", txOut.Hash.String()).
		Msg("Owner unlinked successfully")
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) UpdateAllowedSigners(signers []common.Address, allowed bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.UpdateAllowedSigners(wrc.EthClient.NewTXOpts(), signers, allowed),
	)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call UpdateAllowedSigners")
		return err
	}

	wrc.Logger.Debug().
		Interface("tx", tx).
		Msg("UpdateAllowedSigners transaction submitted")

	if err := wrc.validateReceiptAndEvent(contract, tx, "UpdateAllowedSigners", "AllowedSignersUpdatedV1"); err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("AllowedSignersUpdatedV1 event validation failed")
		return err
	}

	wrc.Logger.Debug().
		Str("txHash", tx.Transaction.Hash().Hex()).
		Msg("Allowed signers updated")
	return nil
}

func (wrc *WorkflowRegistryV2Client) SetDonLimit(donFamily string, limit uint32, userDefaultLimit uint32) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetDONLimit(wrc.EthClient.NewTXOpts(), donFamily, limit, userDefaultLimit),
	)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call SetDonLimit")
		return err
	}

	if tx.Receipt.Status == types.ReceiptStatusFailed {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Msg("SetDonLimit transaction failed")
		return errors.New("SetDonLimit transaction failed")
	}

	wrc.Logger.Debug().
		Str("txHash", tx.Transaction.Hash().Hex()).
		Msg("DON limit set")
	return nil
}

func (wrc *WorkflowRegistryV2Client) SetDONOverride(donFamily [32]byte, limit uint32, enabled bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetUserDONOverride(wrc.EthClient.NewTXOpts(), common.Address{}, common.Hash(donFamily).Hex(), limit, enabled),
	)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call SetDONOverride")
		return err
	}

	if tx.Receipt.Status == types.ReceiptStatusFailed {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Msg("SetDONOverride transaction failed")
		return errors.New("SetDONOverride transaction failed")
	}

	wrc.Logger.Debug().
		Str("txHash", tx.Transaction.Hash().Hex()).
		Msg("DON override set")
	return nil
}

func (wrc *WorkflowRegistryV2Client) SetDefaults(maxPerDON, maxPerUserDON uint32) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(&types.Transaction{}, nil)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to decode SetDefaults transaction")
		return err
	}

	if tx.Receipt.Status == types.ReceiptStatusFailed {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Msg("SetDefaults transaction failed")
		return errors.New("SetDefaults transaction failed")
	}

	wrc.Logger.Debug().
		Str("txHash", tx.Transaction.Hash().Hex()).
		Msg("Defaults set")
	return nil
}

func (wrc *WorkflowRegistryV2Client) SetUserDONOverride(user common.Address, donFamily [32]byte, limit uint32, enabled bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetUserDONOverride(wrc.EthClient.NewTXOpts(), user, common.Hash(donFamily).Hex(), limit, enabled),
	)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call SetUserDONOverride")
		return err
	}

	if tx.Receipt.Status == types.ReceiptStatusFailed {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Msg("SetUserDONOverride transaction failed")
		return errors.New("SetUserDONOverride transaction failed")
	}

	wrc.Logger.Debug().
		Str("txHash", tx.Transaction.Hash().Hex()).
		Msg("User DON override set")
	return nil
}

func (wrc *WorkflowRegistryV2Client) CanLinkOwner(owner common.Address, validityTimestamp *big.Int, proof [32]byte, signature []byte) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for CanLinkOwner")
		return err
	}

	_, err = callContractMethodV2(wrc, func() (struct{}, error) {
		return struct{}{}, contract.CanLinkOwner(wrc.EthClient.NewCallOpts(), owner, validityTimestamp, proof, signature)
	})
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Str("owner", owner.Hex()).
			Msg("CanLinkOwner call failed")
		return err
	}

	wrc.Logger.Debug().
		Str("owner", owner.Hex()).
		Msg("CanLinkOwner call succeeded")

	return nil
}

func (wrc *WorkflowRegistryV2Client) CanUnlinkOwner(owner common.Address, validityTimestamp *big.Int, signature []byte) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for CanUnlinkOwner")
		return err
	}

	_, err = callContractMethodV2(wrc, func() (struct{}, error) {
		return struct{}{}, contract.CanUnlinkOwner(wrc.EthClient.NewCallOpts(), owner, validityTimestamp, signature)
	})
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Str("owner", owner.Hex()).
			Msg("CanUnlinkOwner call failed")
		return err
	}

	wrc.Logger.Debug().
		Str("owner", owner.Hex()).
		Msg("CanUnlinkOwner call succeeded")

	return nil
}

func (wrc *WorkflowRegistryV2Client) GetLinkedOwners(start, batchSize *big.Int) ([]common.Address, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetLinkedOwners")
		return nil, err
	}
	addrs, err := contract.GetLinkedOwners(wrc.EthClient.NewCallOpts(), start, batchSize)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetLinkedOwners call failed")
	}
	return addrs, err
}

func (wrc *WorkflowRegistryV2Client) GetMaxWorkflowsPerDON(donFamily [32]byte) (uint32, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetMaxWorkflowsPerDON")
		return 0, err
	}
	val, err := contract.GetMaxWorkflowsPerDON(wrc.EthClient.NewCallOpts(), common.Hash(donFamily).Hex())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetMaxWorkflowsPerDON call failed")
	}
	return val.MaxWorkflows, err
}

func (wrc *WorkflowRegistryV2Client) GetMaxWorkflowsPerUserDON(user common.Address, donFamily [32]byte) (uint32, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetMaxWorkflowsPerUserDON")
		return 0, err
	}
	val, err := contract.GetMaxWorkflowsPerUserDON(wrc.EthClient.NewCallOpts(), user, common.Hash(donFamily).Hex())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetMaxWorkflowsPerUserDON call failed")
	}
	return val, err
}

func (wrc *WorkflowRegistryV2Client) IsAllowedSigner(signer common.Address) (bool, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for IsAllowedSigner")
		return false, err
	}
	ok, err := contract.IsAllowedSigner(wrc.EthClient.NewCallOpts(), signer)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("IsAllowedSigner call failed")
	}
	return ok, err
}

func (wrc *WorkflowRegistryV2Client) IsOwnerLinked(owner common.Address) (bool, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for IsOwnerLinked")
		return false, err
	}

	result, err := callContractMethodV2(wrc, func() (bool, error) {
		return contract.IsOwnerLinked(wrc.EthClient.NewCallOpts(), owner)
	})
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Str("owner", owner.Hex()).
			Msg("IsOwnerLinked call failed")
		return false, err
	}

	wrc.Logger.Debug().
		Str("owner", owner.Hex()).
		Bool("linked", result).
		Msg("IsOwnerLinked call succeeded")

	return result, nil
}

func (wrc *WorkflowRegistryV2Client) TotalLinkedOwners() (*big.Int, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for TotalLinkedOwners")
		return nil, err
	}
	total, err := contract.TotalLinkedOwners(wrc.EthClient.NewCallOpts())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("TotalLinkedOwners call failed")
	}
	return total, err
}

func (wrc *WorkflowRegistryV2Client) TypeAndVersion() (string, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for TypeAndVersion")
		return "", err
	}
	tv, err := contract.TypeAndVersion(wrc.EthClient.NewCallOpts())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("TypeAndVersion call failed")
	}
	return tv, err
}

func (wrc *WorkflowRegistryV2Client) UpsertWorkflow(params RegisterWorkflowV2Parameters) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.UpsertWorkflow(
			opts,
			params.WorkflowName,
			params.Tag,
			params.WorkflowID,
			params.Status,
			params.DonFamily,
			params.BinaryURL,
			params.ConfigURL,
			params.Attributes,
			params.KeepAlive,
		)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "UpsertWorkflow", "WorkflowRegistered|WorkflowUpdated",
		params.WorkflowName,
		params.Tag,
		params.WorkflowID,
		params.Status,
		params.DonFamily,
		params.BinaryURL,
		params.ConfigURL,
		params.Attributes,
		params.KeepAlive,
	)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call UpsertWorkflow")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) GetWorkflow(owner common.Address, workflowName, tag string) (workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetWorkflow")
		return workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{}, err
	}

	result, err := callContractMethodV2(wrc, func() (workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
		return contract.GetWorkflow(wrc.EthClient.NewCallOpts(), owner, workflowName, tag)
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetWorkflow call failed")
	}
	return result, err
}

func (wrc *WorkflowRegistryV2Client) GetWorkflowListByOwnerAndName(owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetWorkflowListByOwnerAndName")
		return nil, err
	}

	result, err := callContractMethodV2(wrc, func() ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
		return contract.GetWorkflowListByOwnerAndName(wrc.EthClient.NewCallOpts(), owner, workflowName, start, limit)
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetWorkflowListByOwnerAndName call failed")
	}
	return result, err
}

func (wrc *WorkflowRegistryV2Client) DeleteWorkflow(workflowID [32]byte) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.DeleteWorkflow(opts, workflowID)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "DeleteWorkflow", "WorkflowDeleted", workflowID)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call DeleteWorkflow")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) BatchPauseWorkflows(workflowIDs [][32]byte) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.BatchPauseWorkflows(
			opts,
			workflowIDs,
		)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "BatchPauseWorkflows", "WorkflowStatusUpdated", workflowIDs)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call BatchPauseWorkflows")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) ActivateWorkflow(workflowID [32]byte, donFamily string) (*TxOutput, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return nil, err
	}

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.ActivateWorkflow(opts, workflowID, donFamily)
	}
	txOut, err := wrc.executeTransactionByTxType(txFn, "ActivateWorkflow", "WorkflowActivated", workflowID, donFamily)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call ActivateWorkflow")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) validateReceiptAndEvent(
	contract *workflow_registry_v2_wrapper.WorkflowRegistry,
	tx *seth.DecodedTransaction,
	contractFunctionName, contractEventName string,
) error {
	if tx.Receipt.Status == types.ReceiptStatusFailed {
		wrc.Logger.Error().
			Str("contractAddress", contract.Address().Hex()).
			Msgf("Transaction receipt indicates failure for %s", contractFunctionName)
		return errors.New("transaction receipt indicates failure")
	}

	abi, err := workflow_registry_v2_wrapper.WorkflowRegistryMetaData.GetAbi()
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msgf("Failed to get ABI for %s", constants.WorkflowRegistryContractName)
		return errors.New("failed to get contract ABI")
	}

	eventExists, _ := cmdCommon.ValidateEventSignature(wrc.Logger, tx, abi.Events[contractEventName])
	if !eventExists {
		wrc.Logger.Error().
			Str("event", contractEventName).
			Msg("Expected event not emitted")
		return errors.New("event not emitted")
	}

	wrc.Logger.Debug().Msg("Event validated successfully")
	return nil
}

// IsRequestAllowlisted queries the registry to check if a given (owner, requestDigest) is allowlisted.
// requestDigestHex may include or omit the 0x prefix.
func (wrc *WorkflowRegistryV2Client) IsRequestAllowlisted(owner common.Address, digest [32]byte) (bool, error) {
	var contract workflowRegistryV2Contract
	if wrc.Wr != nil {
		contract = wrc.Wr
	} else {
		c, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
		if err != nil {
			wrc.Logger.Error().Err(err).Msg("Failed to connect for IsRequestAllowlisted")
			return false, err
		}
		contract = c
	}

	var allowlisted bool
	_, err := callContractMethodV2(wrc, func() (string, error) {
		var callErr error
		allowlisted, callErr = contract.IsRequestAllowlisted(wrc.EthClient.NewCallOpts(), owner, digest)
		return "", callErr
	})
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Str("owner", owner.Hex()).
			Msg("IsRequestAllowlisted call failed")
		return false, err
	}

	wrc.Logger.Debug().
		Str("owner", owner.Hex()).
		Str("digest", hex.EncodeToString(digest[:])).
		Bool("allowlisted", allowlisted).
		Msg("IsRequestAllowlisted query succeeded")

	return allowlisted, nil
}

// AllowlistRequest sends the request digest to the WorkflowRegistry allowlist with a default expiry of now + 10 minutes.
// `requestDigestHex` should be the hex string produced by utils.CalculateRequestDigest(...), with or without "0x".
func (wrc *WorkflowRegistryV2Client) AllowlistRequest(requestDigest [32]byte, duration time.Duration) error {
	var contract workflowRegistryV2Contract
	if wrc.Wr != nil {
		contract = wrc.Wr
	} else {
		c, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
		if err != nil {
			wrc.Logger.Error().Err(err).Msg("Failed to connect for AllowlistRequest")
			return err
		}
		contract = c
	}

	// #nosec G115 -- int64 to uint32 conversion; Unix() returns seconds since epoch, which fits in uint32 until 2106
	deadline := uint32(time.Now().Add(duration).Unix())

	// Send tx; keep the same "callContractMethodV2" pattern you used for read-only calls.
	// Here we return the tx hash string to the helper (it may log/track it).
	_, err := callContractMethodV2(wrc, func() (string, error) {
		tx, txErr := contract.AllowlistRequest(wrc.EthClient.NewTXOpts(), requestDigest, deadline)
		if txErr != nil {
			return "", txErr
		}
		// Return the tx hash string for visibility through the helper
		return tx.Hash().Hex(), nil
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("AllowlistRequest tx failed")
		return err
	}

	wrc.Logger.Debug().
		Str("digest", hex.EncodeToString(requestDigest[:])).
		Str("deadline", time.Unix(int64(deadline), 0).UTC().Format(time.RFC3339)).
		Msg("AllowlistRequest submitted")
	return nil
}

func callContractMethodV2[T any](wrc *WorkflowRegistryV2Client, contractMethod func() (T, error)) (T, error) {
	result, err := contractMethod()
	if err != nil {
		var zero T
		wrc.Logger.Error().Err(err).Msg("call to contract failed")
		return zero, wrc.EthClient.DecodeSendErr(err)
	}
	return result, nil
}
