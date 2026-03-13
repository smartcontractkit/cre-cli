package bindings

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings/abigen"
)

//go:embed sourcecre.go.tpl
var tpl string

//go:embed mockcontract.go.tpl
var mockTpl string

//go:embed sourcecre.ts.tpl
var tsTpl string

//go:embed mockcontract.ts.tpl
var tsMockTpl string

// extractABI normalises ABI content. If the bytes are a JSON object with an
// "abi" field (e.g. a Solidity compiler artifact), we extract just the array.
// Otherwise the content is returned unchanged (assumed to be a raw ABI array).
func extractABI(data []byte) ([]byte, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, errors.New("empty ABI content")
	}
	// Fast path: if the top-level value is an array, it's already a raw ABI.
	if data[0] == '[' {
		return data, nil
	}
	// Try parsing as an artifact object with an "abi" key.
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("content is neither a JSON array nor a valid JSON object: %w", err)
	}
	if artifact.ABI == nil {
		return nil, errors.New("JSON object does not contain an \"abi\" field")
	}
	return artifact.ABI, nil
}

func GenerateBindings(
	combinedJSONPath string, // path to combined-json, or ""
	abiPath string, // path to a single ABI JSON, or ""
	pkgName string, // generated Go package name
	typeName string, // Go struct name for single-ABI mode (defaults to pkgName)
	outPath string, // where to write the .go file
) error {
	var (
		types   []string
		abis    []string
		bins    []string
		libs    = make(map[string]string)
		aliases = make(map[string]string)
	)

	switch {
	case combinedJSONPath != "":
		// Combined-JSON mode
		data, err := os.ReadFile(combinedJSONPath) //nolint:gosec // G703 -- path from trusted CLI flags
		if err != nil {
			return fmt.Errorf("read combined-json %q: %w", combinedJSONPath, err)
		}
		contracts, err := compiler.ParseCombinedJSON(data, "", "", "", "")
		if err != nil {
			return fmt.Errorf("parse combined-json %q: %w", combinedJSONPath, err)
		}
		for name, c := range contracts {
			parts := strings.Split(name, ":")
			tn := parts[len(parts)-1]
			abiDef, err := json.Marshal(c.Info.AbiDefinition)
			if err != nil {
				return fmt.Errorf("marshal ABI for %s: %w", name, err)
			}
			types = append(types, tn)
			abis = append(abis, string(abiDef))
			bins = append(bins, c.Code)

			// library placeholders
			prefix := crypto.Keccak256Hash([]byte(name)).String()[2:36]
			libs[prefix] = tn
		}

	case abiPath != "":
		// Single-ABI mode
		raw, err := os.ReadFile(abiPath) //nolint:gosec // G703 -- path from trusted CLI flags
		if err != nil {
			return fmt.Errorf("read ABI %q: %w", abiPath, err)
		}
		abiBytes, err := extractABI(raw)
		if err != nil {
			return fmt.Errorf("invalid ABI in %q: %w", abiPath, err)
		}
		// validate that the extracted content is valid JSON
		if err := json.Unmarshal(abiBytes, new(interface{})); err != nil {
			return fmt.Errorf("invalid ABI JSON %q: %w", abiPath, err)
		}
		if typeName == "" {
			typeName = pkgName
		}
		types = []string{typeName}
		abis = []string{string(abiBytes)}
		bins = []string{""} // no deploy bytecode
		// no libraries in single-ABI mode

	default:
		return errors.New("must provide either combinedJSONPath or abiPath")
	}

	// Generate regular bindings w/ forked abigen
	outSrc, err := abigen.BindV2(types, abis, bins, pkgName, libs, aliases, tpl)
	if err != nil {
		return fmt.Errorf("BindV2: %w", err)
	}

	// Write regular bindings file
	if err := os.WriteFile(outPath, []byte(outSrc), 0o600); err != nil { //nolint:gosec // G703 -- path from trusted CLI flags
		return fmt.Errorf("write %q: %w", outPath, err)
	}

	// Generate mock bindings
	mockSrc, err := abigen.BindV2(types, abis, bins, pkgName, libs, aliases, mockTpl)
	if err != nil {
		return fmt.Errorf("BindV2 mock: %w", err)
	}

	// Write mock file with "_mock.go" suffix
	mockPath := strings.TrimSuffix(outPath, ".go") + "_mock.go"
	if err := os.WriteFile(mockPath, []byte(mockSrc), 0o600); err != nil { //nolint:gosec // G703 -- derived from trusted CLI path
		return fmt.Errorf("write mock %q: %w", mockPath, err)
	}

	return nil
}

func GenerateBindingsTS(
	abiPath string,
	typeName string,
	outPath string,
) error {
	if abiPath == "" {
		return errors.New("must provide abiPath")
	}

	raw, err := os.ReadFile(abiPath) //nolint:gosec // G703 -- path from trusted CLI flags
	if err != nil {
		return fmt.Errorf("read ABI %q: %w", abiPath, err)
	}
	abiBytes, err := extractABI(raw)
	if err != nil {
		return fmt.Errorf("invalid ABI in %q: %w", abiPath, err)
	}
	if err := json.Unmarshal(abiBytes, new(interface{})); err != nil {
		return fmt.Errorf("invalid ABI JSON %q: %w", abiPath, err)
	}

	types := []string{typeName}
	abis := []string{string(abiBytes)}
	bins := []string{""}

	libs := make(map[string]string)
	aliases := make(map[string]string)

	outSrc, err := abigen.BindV2TS(types, abis, bins, "", libs, aliases, tsTpl)
	if err != nil {
		return fmt.Errorf("BindV2TS: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(outSrc), 0o600); err != nil { //nolint:gosec // G703 -- path from trusted CLI flags
		return fmt.Errorf("write %q: %w", outPath, err)
	}

	mockSrc, err := abigen.BindV2TS(types, abis, bins, "", libs, aliases, tsMockTpl)
	if err != nil {
		return fmt.Errorf("BindV2TS mock: %w", err)
	}

	mockPath := strings.TrimSuffix(outPath, ".ts") + "_mock.ts"
	if err := os.WriteFile(mockPath, []byte(mockSrc), 0o600); err != nil { //nolint:gosec // G703 -- derived from trusted CLI path
		return fmt.Errorf("write mock %q: %w", mockPath, err)
	}

	return nil
}
