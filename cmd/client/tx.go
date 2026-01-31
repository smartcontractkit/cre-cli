package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

//go:generate stringer -type=TxType
type TxType int

const (
	Regular TxType = iota
	Raw
	Ledger
	Changeset
)

type TxClientConfig struct {
	TxType       TxType
	LedgerConfig *LedgerConfig
	SkipPrompt   bool
}

type TxClient struct {
	Logger    *zerolog.Logger
	EthClient *seth.Client
	abi       *abi.ABI
	config    TxClientConfig
}

// Helper function to check transaction receipt and if the correct event was emitted or not
func (c *TxClient) validateReceiptAndEvent(contractAddr string, tx *seth.DecodedTransaction,
	contractFunctionName string, contractEventNames []string,
) error {
	// check if the transaction receipt is returned
	if tx.Receipt.Status == types.ReceiptStatusFailed {
		c.Logger.Error().
			Str("Contract Address", contractAddr).
			Msgf("Transaction receipt not found for %s function on %s contract", contractFunctionName, constants.WorkflowRegistryContractName)
		return errors.New("transaction receipt not found")
	}

	// check if any of the events were emitted and print out the contents
	for _, eventName := range contractEventNames {
		eventExists, _ := cmdCommon.ValidateEventSignature(c.Logger, tx, c.abi.Events[eventName])
		if eventExists {
			return nil
		}
	}
	return errors.New("none of the specified events were emitted")
}

type TxOutput struct {
	Type  TxType
	Hash  common.Hash
	RawTx RawTx
}

type RawTx struct {
	To       string
	Data     []byte
	Function string
	Args     []string
}

//func (c *TxClient) ledgerOpts(ledgerConfig LedgerConfig) (*bind.TransactOpts, error) {
//	txOpts := &bind.TransactOpts{
//		Value:     big.NewInt(0),
//		GasPrice:  nil,
//		GasFeeCap: nil,
//		GasTipCap: nil,
//		GasLimit:  500_000, //TODO make this configurable
//	}
//	if ledgerConfig.LedgerEnabled {
//		c.Logger.Info().Msg("Ledger mode enabled, preparing to sign transaction with ledger")
//		l, err := NewLedgerClient(c.Logger, ledgerConfig.DerivationPath)
//		if err != nil {
//			return nil, err
//		}
//		nonce, err := c.EthClient.Client.PendingNonceAt(c.EthClient.Context, l.address)
//		if err != nil {
//			return nil, err
//		}
//		txOpts.From = l.address
//		txOpts.Nonce = new(big.Int).SetUint64(nonce)
//
//		c.Logger.Debug().Uint64("Nonce", nonce).Str("From", l.address.Hex()).Msg("Ledger details")
//
//		txOpts.Signer = func(addr common.Address, rawTx *types.Transaction) (*types.Transaction, error) {
//			if addr != l.address {
//				return nil, fmt.Errorf("signer address mismatch: expected %s, got %s", l.address.Hex(), addr.Hex())
//			}
//			signedTx, err := l.SignTransactionWithLedger(rawTx, big.NewInt(c.EthClient.ChainID))
//			if err != nil {
//				return nil, err
//			}
//			return signedTx, nil
//		}
//
//	}
//	return txOpts, nil
//}

func (c *TxClient) executeTransactionByTxType(txFn func(opts *bind.TransactOpts) (*types.Transaction, error), funName string, validationEvent string, args ...any) (TxOutput, error) {
	switch c.config.TxType {
	case Regular:
		simulateTx, err := txFn(cmdCommon.SimTransactOpts())
		if err != nil {
			return TxOutput{Type: Regular}, err
		}
		chainDetails, err := chainselectors.GetChainDetailsByChainIDAndFamily(strconv.FormatInt(c.EthClient.ChainID, 10), chainselectors.FamilyEVM)
		if err != nil {
			return TxOutput{Type: Regular}, err
		}
		msg := ethereum.CallMsg{
			From:     c.EthClient.Addresses[0],
			To:       simulateTx.To(),
			Gas:      0,
			GasPrice: nil,
			Value:    simulateTx.Value(),
			Data:     simulateTx.Data(),
		}
		estimatedGas, gasErr := c.EthClient.Client.EstimateGas(c.EthClient.Context, msg)
		if gasErr != nil {
			c.Logger.Warn().Err(gasErr).Msg("Failed to estimate gas usage")
		}

		ui.Line()
		ui.Title("Transaction details:")
		ui.Printf("  Chain:    %s\n", ui.RenderBold(chainDetails.ChainName))
		ui.Printf("  To:       %s\n", ui.RenderCode(simulateTx.To().Hex()))
		ui.Printf("  Function: %s\n", ui.RenderBold(funName))
		ui.Print("  Inputs:")
		for i, arg := range cmdCommon.ToStringSlice(args) {
			ui.Printf("    [%d]: %s\n", i, arg)
		}
		ui.Line()
		ui.Print("  Data (for verification):")
		ui.Code(fmt.Sprintf("%x", simulateTx.Data()))
		ui.Line()

		// Calculate and print total cost for sending the transaction on-chain
		if gasErr == nil {
			gasPriceWei, gasPriceErr := c.EthClient.Client.SuggestGasPrice(c.EthClient.Context)
			if gasPriceErr != nil {
				c.Logger.Warn().Err(gasPriceErr).Msg("Failed to fetch gas price")
			} else {
				gasPriceGwei := new(big.Float).Quo(new(big.Float).SetInt(gasPriceWei), big.NewFloat(1e9))
				totalCost := new(big.Int).Mul(new(big.Int).SetUint64(estimatedGas), gasPriceWei)
				// Convert from wei to ether for display
				etherValue := new(big.Float).Quo(new(big.Float).SetInt(totalCost), big.NewFloat(1e18))

				ui.Title("Estimated Cost:")
				ui.Printf("  Gas Price:  %s gwei\n", gasPriceGwei.Text('f', 8))
				ui.Printf("  Total Cost: %s\n", ui.RenderBold(etherValue.Text('f', 8)+" ETH"))
			}
		}
		ui.Line()

		// Ask for user confirmation before executing the transaction
		if !c.config.SkipPrompt {
			var confirm bool
			confirmForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Do you want to execute this transaction?").
						Value(&confirm),
				),
			).WithTheme(ui.ChainlinkTheme())
			if err := confirmForm.Run(); err != nil {
				return TxOutput{}, err
			}
			if !confirm {
				return TxOutput{}, errors.New("transaction cancelled by user")
			}
		}

		spinner := ui.NewSpinner()
		spinner.Start("Submitting transaction...")

		decodedTx, err := c.EthClient.Decode(txFn(c.EthClient.NewTXOpts()))
		if err != nil {
			spinner.Stop()
			return TxOutput{Type: Regular}, err
		}
		c.Logger.Debug().Interface("tx", decodedTx.Transaction).Str("TxHash", decodedTx.Transaction.Hash().Hex()).Msg("Transaction mined successfully")

		spinner.Update("Validating transaction...")
		err = c.validateReceiptAndEvent(decodedTx.Transaction.To().Hex(), decodedTx, funName, strings.Split(validationEvent, "|"))
		if err != nil {
			spinner.Stop()
			return TxOutput{Type: Regular}, err
		}
		spinner.Stop()
		return TxOutput{
			Type: Regular,
			Hash: decodedTx.Transaction.Hash(),
			RawTx: RawTx{
				To:       decodedTx.Transaction.To().Hex(),
				Data:     decodedTx.Transaction.Data(),
				Function: funName,
				Args:     cmdCommon.ToStringSlice(args),
			},
		}, nil
	case Raw:
		ui.Warning("--unsigned flag detected: transaction not sent on-chain.")
		ui.Dim("Generating call data for offline signing and submission in your preferred tool:")
		tx, err := txFn(cmdCommon.SimTransactOpts())
		if err != nil {
			return TxOutput{Type: Raw}, err
		}
		c.Logger.Debug().Msgf("Generated call data:\n%s", func() string {
			b, err := json.MarshalIndent(tx, "", "  ")
			if err != nil {
				return fmt.Sprintf("failed to marshal tx: %v", err)
			}
			return string(b)
		}())
		return TxOutput{
			Type: Raw,
			RawTx: RawTx{
				To:       tx.To().Hex(),
				Data:     tx.Data(),
				Function: funName,
				Args:     cmdCommon.ToStringSlice(args),
			},
		}, nil
	case Changeset:
		tx, err := txFn(cmdCommon.SimTransactOpts())
		if err != nil {
			return TxOutput{Type: Changeset}, err
		}
		return TxOutput{
			Type: Changeset,
			RawTx: RawTx{
				To:       tx.To().Hex(),
				Data:     []byte{},
				Function: funName,
				Args:     cmdCommon.ToStringSlice(args),
			},
		}, nil
	//case Ledger:
	//	txOpts, err := c.ledgerOpts(c.ledgerConfig)
	//	if err != nil {
	//		return TxOutput{Type: Ledger}, err
	//	}
	//	// seth.Decode doesn't work with ledger, it always requires private key,
	//	//so we mine the txn and then use seth.DecodeSendErr and then seth.DecodeTx
	//	tx, err := txFn(txOpts)
	//	if err != nil {
	//		return TxOutput{Type: Ledger}, c.EthClient.DecodeSendErr(err)
	//	}
	//	decodedTx, err := c.EthClient.DecodeTx(tx)
	//	if err != nil {
	//		return TxOutput{Type: Ledger}, err
	//	}
	//	err = c.validateReceiptAndEvent(decodedTx.Transaction.To().Hex(), decodedTx, funName, validationEvent)
	//	if err != nil {
	//		return TxOutput{}, err
	//	}
	//	return TxOutput{Type: Ledger, Hash: tx.Hash()}, nil
	default:
		return TxOutput{}, fmt.Errorf("unknown output type: %d", c.config.TxType)
	}
}
