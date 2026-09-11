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
	cmd := exec.Command("go", "test", "./"+filepath.ToSlash(relPkgDir), "-run", "TestPrintReportPayload", "-v")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return extractPayloadHex(t, string(output))
}

func generatedTSReportPayloadHex(t *testing.T, repoRoot, tsFile string) string {
	t.Helper()

	writeGeneratedTSSDKStub(t, filepath.Dir(tsFile))

	jsOutDir := filepath.Join(filepath.Dir(tsFile), "js")
	cmd := exec.Command(
		filepath.Join(repoRoot, "node_modules", ".bin", "tsc"),
		tsFile,
		"--target", "ES2022",
		"--module", "NodeNext",
		"--moduleResolution", "NodeNext",
		"--outDir", jsOutDir,
		"--skipLibCheck",
	)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))

	compiledFile := filepath.Join(jsOutDir, filepath.Base(strings.TrimSuffix(tsFile, ".ts")+".js"))
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
export const zeroAddress = '0x0000000000000000000000000000000000000000'
export const LAST_FINALIZED_BLOCK_NUMBER = {}
export class EVMClient {}
export const bytesToHex = (bytes) => '0x' + Buffer.from(bytes).toString('hex')
export const hexToBase64 = (hex) => Buffer.from(hex.slice(2), 'hex').toString('base64')
export const encodeCallMsg = (call) => call
export const prepareReportRequest = (hexPayload) => ({
  encodedPayload: Buffer.from(hexPayload.slice(2), 'hex').toString('base64'),
  encoderName: 'evm',
  signingAlgo: 'ecdsa',
  hashingAlgo: 'keccak256',
})
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sdkDir, "index.d.ts"), []byte(`
export type Runtime<T> = {
  report(request: unknown): { result(): unknown }
}
export type EVMLog = { data: Uint8Array; topics: Uint8Array[] }
export declare const zeroAddress: '0x0000000000000000000000000000000000000000'
export declare const LAST_FINALIZED_BLOCK_NUMBER: unknown
export declare class EVMClient {
  callContract(runtime: unknown, input: unknown): { result(): { data: Uint8Array } }
  writeReport(runtime: unknown, input: unknown): { result(): unknown }
}
export declare const bytesToHex: (bytes: Uint8Array) => `+"`0x${string}`"+`
export declare const hexToBase64: (hex: `+"`0x${string}`"+`) => string
export declare const encodeCallMsg: <T>(call: T) => T
export declare const prepareReportRequest: (hexPayload: `+"`0x${string}`"+`) => {
  encodedPayload: string
  encoderName: string
  signingAlgo: string
  hashingAlgo: string
}
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
