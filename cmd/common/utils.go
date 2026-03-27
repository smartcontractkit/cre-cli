package common

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"sigs.k8s.io/yaml"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/context"
	"github.com/smartcontractkit/cre-cli/internal/logger"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	inttypes "github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
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

// GetWorkflowLanguage determines the workflow language based on the file extension
// Note: inputFile can be a file path (e.g., "main.ts", "main.go", or "workflow.wasm") or a directory (for Go workflows, e.g., ".")
// Returns constants.WorkflowLanguageTypeScript for .ts or .tsx files, constants.WorkflowLanguageWasm for .wasm files, constants.WorkflowLanguageGolang otherwise
func GetWorkflowLanguage(inputFile string) string {
	if strings.HasSuffix(inputFile, ".ts") || strings.HasSuffix(inputFile, ".tsx") {
		return constants.WorkflowLanguageTypeScript
	}
	if strings.HasSuffix(inputFile, ".wasm") {
		return constants.WorkflowLanguageWasm
	}
	return constants.WorkflowLanguageGolang
}

// ResolveWorkflowPath turns a workflow-path value from YAML (e.g. "." or "main.ts") into an
// absolute path to the main file. When pathFromYAML is "." or "", looks for main.go then main.ts
// under workflowDir. Callers can use GetWorkflowLanguage on the result to get the language.
func ResolveWorkflowPath(workflowDir, pathFromYAML string) (absPath string, err error) {
	workflowDir, err = filepath.Abs(workflowDir)
	if err != nil {
		return "", fmt.Errorf("workflow directory: %w", err)
	}
	if pathFromYAML == "" || pathFromYAML == "." {
		mainGo := filepath.Join(workflowDir, "main.go")
		mainTS := filepath.Join(workflowDir, "main.ts")
		if _, err := os.Stat(mainGo); err == nil {
			return mainGo, nil
		}
		if _, err := os.Stat(mainTS); err == nil {
			return mainTS, nil
		}
		return "", fmt.Errorf("no main.go or main.ts in %s", workflowDir)
	}
	joined := filepath.Join(workflowDir, pathFromYAML)
	return filepath.Abs(joined)
}

// WorkflowPathRootAndMain returns the absolute root directory and main file name for a workflow
// path (e.g. "workflowName/main.go" -> rootDir, "main.go"). Use with GetWorkflowLanguage(mainFile)
// for consistent language detection.
func WorkflowPathRootAndMain(workflowPath string) (rootDir, mainFile string, err error) {
	abs, err := filepath.Abs(workflowPath)
	if err != nil {
		return "", "", fmt.Errorf("workflow path: %w", err)
	}
	return filepath.Dir(abs), filepath.Base(abs), nil
}

// EnsureTool checks that the binary exists on PATH
func EnsureTool(bin string) error {
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%q not found in PATH: %w", bin, err)
	}
	return nil
}

func WriteChangesetFile(fileName string, changesetFile *inttypes.ChangesetFile, settings *settings.Settings) error {
	// Set project context so the changeset path is resolved from project root
	if err := context.SetProjectContext(""); err != nil {
		return err
	}

	fullFilePath := filepath.Join(
		filepath.Clean(settings.CLDSettings.CLDPath),
		"domains",
		settings.CLDSettings.Domain,
		settings.CLDSettings.Environment,
		"durable_pipelines",
		"inputs",
		fileName,
	)

	// if file exists, read it and append the new changesets
	if _, err := os.Stat(fullFilePath); err == nil {
		existingYamlData, err := os.ReadFile(fullFilePath)
		if err != nil {
			return fmt.Errorf("failed to read existing changeset yaml file: %w", err)
		}

		var existingChangesetFile inttypes.ChangesetFile
		if err := yaml.Unmarshal(existingYamlData, &existingChangesetFile); err != nil {
			return fmt.Errorf("failed to unmarshal existing changeset yaml: %w", err)
		}

		// Append new changesets to the existing ones
		existingChangesetFile.Changesets = append(existingChangesetFile.Changesets, changesetFile.Changesets...)
		changesetFile = &existingChangesetFile
	}

	yamlData, err := yaml.Marshal(&changesetFile)
	if err != nil {
		return fmt.Errorf("failed to marshal changeset to yaml: %w", err)
	}

	if err := os.WriteFile(fullFilePath, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write changeset yaml file: %w", err)
	}

	ui.Line()
	ui.Success("Changeset YAML file generated!")
	ui.Code(fullFilePath)
	ui.Line()
	return nil
}
