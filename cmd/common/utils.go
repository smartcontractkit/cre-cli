package common

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/logger"
)

func ValidateEventSignature(l *zerolog.Logger, tx *seth.DecodedTransaction, e abi.Event) (bool, int) {
	eventValidated := false
	num := 0

	for _, event := range tx.Events {
		l.Debug().
			Object("Event", logger.DecodedTransactionLogWrapper{DecodedTransactionLog: event}).
			Msg("Found event")
		if strings.Contains(event.Signature, e.RawName) {
			l.Debug().
				Object("Event Data", logger.EventDataWrapper{EventData: event.EventData}).
				Str("Transaction", tx.Transaction.Hash().Hex()).
				Msgf("%s event emitted", e.RawName)
			eventValidated = true
			num++
		}
	}

	if !eventValidated {
		l.Debug().Msgf("%s event not emitted", e.RawName)
	}
	return eventValidated, num
}

// SimTransactOpts is useful to generate just the calldata for a given gethwrapper method.
func SimTransactOpts() *bind.TransactOpts {
	return &bind.TransactOpts{Signer: func(address common.Address, transaction *types.Transaction) (*types.Transaction, error) {
		return transaction, nil
	}, From: common.HexToAddress("0x0"), NoSend: true, GasLimit: 1_000_000}
}

func WriteJsonToFile(j interface{}, filePath string) error {
	jsonBytes, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonBytes, 0600)
}

func GetDirectoryName() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return filepath.Base(wd), nil
}

func MustGetUserInputWithPrompt(l *zerolog.Logger, prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	l.Info().Msg(prompt)
	var input string

	for attempt := 0; attempt < 5; attempt++ {
		var err error
		input, err = reader.ReadString('\n')
		if err != nil {
			l.Info().Msg("✋ Failed to read user input, please try again.")
		}
		if input != "\n" {
			return strings.TrimRight(input, "\n"), nil
		}
		l.Info().Msg("✋ Invalid input, please try again")
	}

	l.Info().Msg("✋ Maximum number of attempts reached, aborting")
	return "", errors.New("maximum attempts reached")
}

func AddTimeStampToFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	name := strings.TrimSuffix(fileName, ext)
	return fmt.Sprintf("%s-%s%s", name, time.Now().UTC().Format(time.RFC3339), ext)
}

func DeleteFileIfExists(filePath string) error {
	if _, err := os.Stat(filePath); err == nil {
		return os.Remove(filePath)
	}
	return nil
}

func ComputeHashKey(owner common.Address, workflowName string) [32]byte {
	// Convert the owner address from hex string to bytes
	ownerBytes := owner.Bytes()

	// Convert the name string to bytes (UTF-8 encoding)
	nameBytes := []byte(workflowName)

	// Concatenate the owner bytes and name bytes (similar to abi.encodePacked)
	data := append(ownerBytes, nameBytes...)

	// Compute the Keccak256 hash
	return crypto.Keccak256Hash(data)
}

// There is only a small group of acceptable file extensions by this tool and only few of them are considered to be binary files
func IsBinaryFile(fileName string) (bool, error) {
	// this is binary wasm file (additional .br extension if it's compressed by Brotli)
	if strings.HasSuffix(fileName, ".wasm.br") ||
		strings.HasSuffix(fileName, ".wasm") {
		return true, nil
		// this is a configuration or secrets file
	} else if strings.HasSuffix(fileName, ".yaml") ||
		strings.HasSuffix(fileName, ".yml") ||
		strings.HasSuffix(fileName, ".json") {
		return false, nil
	}
	return false, fmt.Errorf("file extension not supported by the tool: %s, supported extensions: .wasm.br, .json, .yaml, .yml", fileName)
}

// toStringSlice converts a slice of any type to a slice of strings.
// If an element is a byte slice, it prints it as hex.
func ToStringSlice(args []any) []string {
	result := make([]string, len(args))
	for i, v := range args {
		switch b := v.(type) {
		case []byte, [32]byte:
			result[i] = fmt.Sprintf("0x%x", b)
		case [][]byte:
			hexStrings := make([]string, len(b))
			for j, bb := range b {
				hexStrings[j] = fmt.Sprintf("0x%x", bb)
			}
			result[i] = fmt.Sprintf("[%s]", strings.Join(hexStrings, ", "))
		case [][32]byte:
			hexStrings := make([]string, len(b))
			for j, bb := range b {
				hexStrings[j] = fmt.Sprintf("0x%x", bb)
			}
			result[i] = fmt.Sprintf("[%s]", strings.Join(hexStrings, ", "))
		default:
			result[i] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
