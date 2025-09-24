package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
)

//go:generate stringer -type=TxType
type TxType int

const (
	Regular TxType = iota
	Raw
	Ledger
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
		c.Logger.Info().Msgf("Transaction details:\n")
		c.Logger.Info().Msgf("  To:   %s\n", simulateTx.To().Hex())
		c.Logger.Info().Msgf("  Function: %s\n", funName)
		c.Logger.Info().Msgf("  Inputs: %s\n", strings.Join(cmdCommon.ToStringSlice(args), ", "))
		c.Logger.Info().Msgf("  Data: %x\n", simulateTx.Data())

		// Ask for user confirmation before executing the transaction
		if !c.config.SkipPrompt {
			confirm, err := prompt.YesNoPrompt(os.Stdin, "Do you want to execute this transaction?")
			if err != nil {
				return TxOutput{}, err
			}
			if !confirm {
				return TxOutput{}, errors.New("transaction cancelled by user")
			}
		}

		decodedTx, err := c.EthClient.Decode(txFn(c.EthClient.NewTXOpts()))
		if err != nil {
			return TxOutput{Type: Regular}, err
		}
		c.Logger.Debug().Interface("tx", decodedTx.Transaction).Str("TxHash", decodedTx.Transaction.Hash().Hex()).Msg("Transaction mined successfully")

		err = c.validateReceiptAndEvent(decodedTx.Transaction.To().Hex(), decodedTx, funName, strings.Split(validationEvent, "|"))
		if err != nil {
			return TxOutput{Type: Regular}, err
		}
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
		c.Logger.Info().Msg("--unsigned flag detected: transaction not sent on-chain.")
		c.Logger.Info().Msg("Generating call data for offline signing and submission in your preferred tool:\n")
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
