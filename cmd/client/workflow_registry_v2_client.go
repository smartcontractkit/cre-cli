package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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

func (wrc *WorkflowRegistryV2Client) callOpts(ctx context.Context) *bind.CallOpts {
	opts := wrc.EthClient.NewCallOpts()
	opts.Context = ctx
	return opts
}

func (wrc *WorkflowRegistryV2Client) txOpts(ctx context.Context) *bind.TransactOpts {
	opts := wrc.EthClient.NewTXOpts()
	opts.Context = ctx
	return opts
}

func (wrc *WorkflowRegistryV2Client) LinkOwner(ctx context.Context, validityTimestamp *big.Int, proof [32]byte, signature []byte) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "LinkOwner", "OwnershipLinkUpdated", validityTimestamp, proof, signature)
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

func (wrc *WorkflowRegistryV2Client) UnlinkOwner(ctx context.Context, owner common.Address, validityTimestamp *big.Int, signature []byte) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "UnlinkOwner", "OwnershipLinkUpdated", owner, validityTimestamp, signature)
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

func (wrc *WorkflowRegistryV2Client) UpdateAllowedSigners(ctx context.Context, signers []common.Address, allowed bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.UpdateAllowedSigners(wrc.txOpts(ctx), signers, allowed),
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

func (wrc *WorkflowRegistryV2Client) SetDonLimit(ctx context.Context, donFamily string, limit uint32, userDefaultLimit uint32) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetDONLimit(wrc.txOpts(ctx), donFamily, limit, userDefaultLimit),
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

func (wrc *WorkflowRegistryV2Client) SetDONOverride(ctx context.Context, donFamily [32]byte, limit uint32, enabled bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetUserDONOverride(wrc.txOpts(ctx), common.Address{}, common.Hash(donFamily).Hex(), limit, enabled),
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

func (wrc *WorkflowRegistryV2Client) SetDefaults(ctx context.Context, maxPerDON, maxPerUserDON uint32) error {
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

func (wrc *WorkflowRegistryV2Client) SetUserDONOverride(ctx context.Context, user common.Address, donFamily [32]byte, limit uint32, enabled bool) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msgf("Failed to connect to %s", constants.WorkflowRegistryContractName)
		return err
	}

	tx, err := wrc.EthClient.Decode(
		contract.SetUserDONOverride(wrc.txOpts(ctx), user, common.Hash(donFamily).Hex(), limit, enabled),
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

func (wrc *WorkflowRegistryV2Client) CanLinkOwner(ctx context.Context, owner common.Address, validityTimestamp *big.Int, proof [32]byte, signature []byte) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for CanLinkOwner")
		return err
	}

	_, err = callContractMethodV2(wrc, func() (struct{}, error) {
		return struct{}{}, contract.CanLinkOwner(wrc.callOpts(ctx), owner, validityTimestamp, proof, signature)
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

func (wrc *WorkflowRegistryV2Client) CanUnlinkOwner(ctx context.Context, owner common.Address, validityTimestamp *big.Int, signature []byte) error {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for CanUnlinkOwner")
		return err
	}

	_, err = callContractMethodV2(wrc, func() (struct{}, error) {
		return struct{}{}, contract.CanUnlinkOwner(wrc.callOpts(ctx), owner, validityTimestamp, signature)
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

func (wrc *WorkflowRegistryV2Client) GetLinkedOwners(ctx context.Context, start, batchSize *big.Int) ([]common.Address, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetLinkedOwners")
		return nil, err
	}
	addrs, err := contract.GetLinkedOwners(wrc.callOpts(ctx), start, batchSize)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetLinkedOwners call failed")
	}
	return addrs, err
}

func (wrc *WorkflowRegistryV2Client) GetMaxWorkflowsPerDON(ctx context.Context, donFamily [32]byte) (uint32, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetMaxWorkflowsPerDON")
		return 0, err
	}
	val, err := contract.GetMaxWorkflowsPerDON(wrc.callOpts(ctx), common.Hash(donFamily).Hex())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetMaxWorkflowsPerDON call failed")
	}
	return val.MaxWorkflows, err
}

func (wrc *WorkflowRegistryV2Client) GetMaxWorkflowsPerUserDON(ctx context.Context, user common.Address, donFamily [32]byte) (uint32, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetMaxWorkflowsPerUserDON")
		return 0, err
	}
	val, err := contract.GetMaxWorkflowsPerUserDON(wrc.callOpts(ctx), user, common.Hash(donFamily).Hex())
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetMaxWorkflowsPerUserDON call failed")
	}
	return val, err
}

func (wrc *WorkflowRegistryV2Client) GetMaxWorkflowsPerUserDONByFamily(ctx context.Context, user common.Address, donFamily string) (uint32, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetMaxWorkflowsPerUserDONByFamily")
		return 0, err
	}
	val, err := contract.GetMaxWorkflowsPerUserDON(wrc.callOpts(ctx), user, donFamily)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetMaxWorkflowsPerUserDONByFamily call failed")
	}
	return val, err
}

func (wrc *WorkflowRegistryV2Client) IsAllowedSigner(ctx context.Context, signer common.Address) (bool, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for IsAllowedSigner")
		return false, err
	}
	ok, err := contract.IsAllowedSigner(wrc.callOpts(ctx), signer)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("IsAllowedSigner call failed")
	}
	return ok, err
}

func (wrc *WorkflowRegistryV2Client) IsOwnerLinked(ctx context.Context, owner common.Address) (bool, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().
			Err(err).
			Msg("Failed to connect for IsOwnerLinked")
		return false, err
	}

	result, err := callContractMethodV2(wrc, func() (bool, error) {
		return contract.IsOwnerLinked(wrc.callOpts(ctx), owner)
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

func (wrc *WorkflowRegistryV2Client) TotalLinkedOwners(ctx context.Context) (*big.Int, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for TotalLinkedOwners")
		return nil, err
	}
	total, err := contract.TotalLinkedOwners(wrc.callOpts(ctx))
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("TotalLinkedOwners call failed")
	}
	return total, err
}

func (wrc *WorkflowRegistryV2Client) TypeAndVersion(ctx context.Context) (string, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for TypeAndVersion")
		return "", err
	}
	tv, err := contract.TypeAndVersion(wrc.callOpts(ctx))
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("TypeAndVersion call failed")
	}
	return tv, err
}

func (wrc *WorkflowRegistryV2Client) UpsertWorkflow(ctx context.Context, params RegisterWorkflowV2Parameters) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "UpsertWorkflow", "WorkflowRegistered|WorkflowUpdated",
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

func (wrc *WorkflowRegistryV2Client) GetWorkflow(ctx context.Context, owner common.Address, workflowName, tag string) (workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetWorkflow")
		return workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView{}, err
	}

	result, err := callContractMethodV2(wrc, func() (workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
		return contract.GetWorkflow(wrc.callOpts(ctx), owner, workflowName, tag)
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetWorkflow call failed")
	}
	return result, err
}

func (wrc *WorkflowRegistryV2Client) GetWorkflowListByOwnerAndName(ctx context.Context, owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetWorkflowListByOwnerAndName")
		return nil, err
	}

	result, err := callContractMethodV2(wrc, func() ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
		return contract.GetWorkflowListByOwnerAndName(wrc.callOpts(ctx), owner, workflowName, start, limit)
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetWorkflowListByOwnerAndName call failed")
	}
	return result, err
}

func (wrc *WorkflowRegistryV2Client) GetWorkflowListByOwner(ctx context.Context, owner common.Address, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	contract, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("Failed to connect for GetWorkflowListByOwner")
		return nil, err
	}

	result, err := callContractMethodV2(wrc, func() ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
		return contract.GetWorkflowListByOwner(wrc.callOpts(ctx), owner, start, limit)
	})
	if err != nil {
		wrc.Logger.Error().Err(err).Msg("GetWorkflowListByOwner call failed")
	}
	return result, err
}

func (wrc *WorkflowRegistryV2Client) CheckUserDonLimit(
	ctx context.Context,
	owner common.Address,
	donFamily string,
	pending uint32,
) error {
	const workflowStatusActive = uint8(0)
	const workflowListPageSize = int64(200)

	maxAllowed, err := wrc.GetMaxWorkflowsPerUserDONByFamily(ctx, owner, donFamily)
	if err != nil {
		return fmt.Errorf("failed to fetch per-user workflow limit: %w", err)
	}

	var currentActive uint32
	start := big.NewInt(0)
	limit := big.NewInt(workflowListPageSize)

	for {
		list, err := wrc.GetWorkflowListByOwner(ctx, owner, start, limit)
		if err != nil {
			return fmt.Errorf("failed to check active workflows for DON %s: %w", donFamily, err)
		}
		if len(list) == 0 {
			break
		}

		for _, workflow := range list {
			if workflow.Status == workflowStatusActive && workflow.DonFamily == donFamily {
				currentActive++
			}
		}

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < workflowListPageSize {
			break
		}
	}

	if currentActive+pending > maxAllowed {
		return fmt.Errorf("workflow limit reached for DON %s: %d/%d active workflows", donFamily, currentActive, maxAllowed)
	}

	return nil
}

func (wrc *WorkflowRegistryV2Client) DeleteWorkflow(ctx context.Context, workflowID [32]byte) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "DeleteWorkflow", "WorkflowDeleted", workflowID)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call DeleteWorkflow")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) BatchPauseWorkflows(ctx context.Context, workflowIDs [][32]byte) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "BatchPauseWorkflows", "WorkflowStatusUpdated", workflowIDs)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", contract.Address().Hex()).
			Err(err).
			Msg("Failed to call BatchPauseWorkflows")
		return nil, err
	}
	return &txOut, nil
}

func (wrc *WorkflowRegistryV2Client) ActivateWorkflow(ctx context.Context, workflowID [32]byte, donFamily string) (*TxOutput, error) {
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
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "ActivateWorkflow", "WorkflowActivated", workflowID, donFamily)
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
func (wrc *WorkflowRegistryV2Client) IsRequestAllowlisted(ctx context.Context, owner common.Address, digest [32]byte) (bool, error) {
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
		allowlisted, callErr = contract.IsRequestAllowlisted(wrc.callOpts(ctx), owner, digest)
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
func (wrc *WorkflowRegistryV2Client) AllowlistRequest(ctx context.Context, requestDigest [32]byte, duration time.Duration) (*TxOutput, error) {
	var contract workflowRegistryV2Contract
	if wrc.Wr != nil {
		contract = wrc.Wr
	} else {
		c, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(wrc.ContractAddress, wrc.EthClient.Client)
		if err != nil {
			wrc.Logger.Error().Err(err).Msg("Failed to connect for AllowlistRequest")
			return nil, err
		}
		contract = c
	}

	// #nosec G115 -- int64 to uint32 conversion; Unix() returns seconds since epoch, which fits in uint32 until 2106
	deadline := uint32(time.Now().Add(duration).Unix())

	txFn := func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return contract.AllowlistRequest(opts, requestDigest, deadline)
	}
	txOut, err := wrc.executeTransactionByTxType(ctx, txFn, "AllowlistRequest", "RequestAllowlisted", requestDigest, duration)
	if err != nil {
		wrc.Logger.Error().
			Str("contract", wrc.ContractAddress.Hex()).
			Err(err).
			Msg("Failed to call AllowlistRequest")
		return nil, err
	}
	wrc.Logger.Debug().
		Str("digest", hex.EncodeToString(requestDigest[:])).
		Str("deadline", time.Unix(int64(deadline), 0).UTC().Format(time.RFC3339)).
		Msg("AllowlistRequest submitted")
	return &txOut, nil
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
