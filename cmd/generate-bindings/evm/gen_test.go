package evm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/evm"
)

func TestGenerateBindings(t *testing.T) {
	if err := evm.GenerateBindings(
		"./testdata/DataStorage_combined.json",
		"",
		"bindings",
		"",
		"./testdata/bindings.go",
	); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateBindingsCrossLanguageReportPayloadGolden(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := filepath.Clean(filepath.Join(wd, "../../.."))

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for TypeScript report payload golden test")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "node_modules", "viem")); err != nil {
		t.Skip("node_modules/viem is required for TypeScript report payload golden test")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "node_modules", "typescript")); err != nil {
		t.Skip("node_modules/typescript is required for TypeScript report payload golden test")
	}

	tempDir, err := os.MkdirTemp(wd, "golden-report-payload-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiContent := `[
		{
			"type": "function",
			"name": "updatePrices",
			"inputs": [{
				"name": "priceData",
				"type": "tuple",
				"internalType": "struct PriceUpdater.PriceData",
				"components": [
					{"name": "ethPrice", "type": "uint256"},
					{"name": "btcPrice", "type": "uint256"}
				]
			}],
			"outputs": [],
			"stateMutability": "nonpayable"
		}
	]`
	abiFile := filepath.Join(tempDir, "PriceUpdater.abi")
	require.NoError(t, os.WriteFile(abiFile, []byte(abiContent), 0o600))

	goOutDir := filepath.Join(tempDir, "priceupdater")
	require.NoError(t, os.MkdirAll(goOutDir, 0o755))
	goOutFile := filepath.Join(goOutDir, "price_updater.go")
	require.NoError(t, evm.GenerateBindings("", abiFile, "priceupdater", "PriceUpdater", goOutFile))

	tsOutFile := filepath.Join(tempDir, "PriceUpdater.ts")
	require.NoError(t, evm.GenerateBindingsTS(abiFile, "PriceUpdater", tsOutFile))

	goPayload := generatedGoReportPayloadHex(t, repoRoot, goOutDir)
	tsPayload := generatedTSReportPayloadHex(t, repoRoot, tsOutFile)

	require.Equal(t, goPayload, tsPayload)
	require.Equal(t, 64, len(goPayload)/2)
	require.NotEqual(t, "6adc10b0", goPayload[:8])
}

func generatedGoReportPayloadHex(t *testing.T, repoRoot, pkgDir string) string {
	t.Helper()

	testFile := filepath.Join(pkgDir, "payload_test.go")
	require.NoError(t, os.WriteFile(testFile, []byte(`package priceupdater

import (
	"fmt"
	"math/big"
	"testing"
)

func TestPrintReportPayload(t *testing.T) {
	codec, err := NewCodec()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := codec.EncodePriceDataStruct(PriceData{
		EthPrice: big.NewInt(123),
		BtcPrice: big.NewInt(456),
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("REPORT_PAYLOAD_HEX=%x\n", payload)
}
`), 0o600))

	relPkgDir, err := filepath.Rel(repoRoot, pkgDir)
	require.NoError(t, err)
	cmd := exec.Command("go", "test", "./"+filepath.ToSlash(relPkgDir), "-run", "TestPrintReportPayload", "-v") //nolint:gosec // G204 -- relPkgDir is a test-created temp package under this repo.
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return extractPayloadHex(t, string(output))
}

func generatedTSReportPayloadHex(t *testing.T, repoRoot, tsFile string) string {
	t.Helper()

	writeGeneratedTSSDKStub(t, filepath.Dir(tsFile))

	jsOutDir := filepath.Join(filepath.Dir(tsFile), "js")
	require.NoError(t, os.MkdirAll(jsOutDir, 0o755))

	compiledFile := filepath.Join(jsOutDir, filepath.Base(strings.TrimSuffix(tsFile, ".ts")+".js"))
	transpileScript := `
import { readFileSync, writeFileSync } from 'node:fs'
import ts from 'typescript'

const src = readFileSync(process.argv[1], 'utf8')
const result = ts.transpileModule(src, {
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
  },
})
writeFileSync(process.argv[2], result.outputText)
`
	cmd := exec.Command("node", "--input-type=module", "-e", transpileScript, tsFile, compiledFile)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	script := `
import { pathToFileURL } from 'node:url'

const { PriceUpdater } = await import(pathToFileURL(process.argv[1]).href)

let capturedRequest
const runtime = {
  report(request) {
    capturedRequest = request
    return { result: () => ({}) }
  },
}
const client = {
  writeReport() {
    return { result: () => ({}) }
  },
}

const binding = new PriceUpdater(client, '0x0000000000000000000000000000000000000001')
binding.writeReportFromUpdatePrices(runtime, { ethPrice: 123n, btcPrice: 456n })

if (!capturedRequest?.encodedPayload) throw new Error('report request payload not captured')
console.log('REPORT_PAYLOAD_HEX=' + Buffer.from(capturedRequest.encodedPayload, 'base64').toString('hex'))
`
	cmd = exec.Command("node", "--input-type=module", "-e", script, compiledFile)
	cmd.Dir = repoRoot
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return extractPayloadHex(t, string(output))
}

func writeGeneratedTSSDKStub(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o600))

	sdkDir := filepath.Join(dir, "node_modules", "@chainlink", "cre-sdk")
	require.NoError(t, os.MkdirAll(sdkDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sdkDir, "package.json"), []byte(`{
  "type": "module",
  "exports": {
    ".": {
      "types": "./index.d.ts",
      "import": "./index.js"
    }
  }
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sdkDir, "index.js"), []byte(`
export const hexToBase64 = (hex) => Buffer.from(hex.slice(2), 'hex').toString('base64')
export const prepareReportRequest = (hexPayload) => ({
  encodedPayload: Buffer.from(hexPayload.slice(2), 'hex').toString('base64'),
  encoderName: 'evm',
  signingAlgo: 'ecdsa',
  hashingAlgo: 'keccak256',
})
`), 0o600))
}

func extractPayloadHex(t *testing.T, output string) string {
	t.Helper()

	for _, line := range strings.Split(output, "\n") {
		if payload, ok := strings.CutPrefix(strings.TrimSpace(line), "REPORT_PAYLOAD_HEX="); ok {
			return payload
		}
	}
	t.Fatalf("REPORT_PAYLOAD_HEX not found in output:\n%s", output)
	return ""
}
