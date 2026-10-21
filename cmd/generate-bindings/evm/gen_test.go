package evm_test

import (
	"fmt"
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

	requireTSReportTestDependencies(t, repoRoot)

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
	require.NoError(t, evm.GenerateBindingsTS(abiFile, "PriceUpdater", tsOutFile, evm.TSBindingOptions{}))

	goPayload := generatedGoReportPayloadHex(t, repoRoot, goOutDir)
	tsPayload := generatedTSReportPayloadHex(t, repoRoot, tsOutFile)

	require.Equal(t, goPayload, tsPayload)
	require.Equal(t, 64, len(goPayload)/2)
	require.NotEqual(t, "6adc10b0", goPayload[:8])

	legacyDir := filepath.Join(tempDir, "legacy")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	legacyFile := filepath.Join(legacyDir, "PriceUpdater.ts")
	require.NoError(t, evm.GenerateBindingsTS(abiFile, "PriceUpdater", legacyFile,
		evm.TSBindingOptions{IncludeFunctionSelector: true}))
	legacyPayload := generatedTSReportPayloadHex(t, repoRoot, legacyFile)
	require.Equal(t, "6adc10b0"+goPayload, legacyPayload, "legacy reports must contain the selector followed by the same encoded arguments")
	require.Equal(t, 68, len(legacyPayload)/2)
}

func TestGenerateBindingsTS_OverloadedReportPayloads(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	repoRoot := filepath.Clean(filepath.Join(wd, "../../.."))
	requireTSReportTestDependencies(t, repoRoot)

	tempDir, err := os.MkdirTemp(wd, "overloaded-report-payload-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	abiFile := filepath.Join(tempDir, "Overloaded.abi")
	require.NoError(t, os.WriteFile(abiFile, []byte(`[
		{"type":"function","name":"setValue","inputs":[{"name":"value","type":"uint128"}],"outputs":[],"stateMutability":"nonpayable"},
		{"type":"function","name":"setValue","inputs":[{"name":"value","type":"uint256"}],"outputs":[],"stateMutability":"nonpayable"},
		{"type":"function","name":"setValue","inputs":[{"name":"value","type":"string"}],"outputs":[],"stateMutability":"nonpayable"},
		{"type":"function","name":"setValue","inputs":[{"name":"values","type":"tuple[]","internalType":"struct Overloaded.Value[]","components":[{"name":"count","type":"uint256"},{"name":"label","type":"string"}]}],"outputs":[],"stateMutability":"nonpayable"}
	]`), 0o600))

	for _, includeSelector := range []bool{false, true} {
		t.Run(fmt.Sprintf("includeSelector=%t", includeSelector), func(t *testing.T) {
			outDir := filepath.Join(tempDir, fmt.Sprintf("selector-%t", includeSelector))
			require.NoError(t, os.MkdirAll(outDir, 0o755))
			tsFile := filepath.Join(outDir, "Overloaded.ts")
			require.NoError(t, evm.GenerateBindingsTS(abiFile, "Overloaded", tsFile,
				evm.TSBindingOptions{IncludeFunctionSelector: includeSelector}))
			compiledFile := compileGeneratedTS(t, repoRoot, tsFile)

			script := `
import assert from 'node:assert/strict'
import { pathToFileURL } from 'node:url'
import { encodeAbiParameters, encodeFunctionData } from 'viem'

const { Overloaded, OverloadedABI } = await import(pathToFileURL(process.argv[1]).href)
const includeSelector = process.argv[2] === 'true'
let capturedRequest
const runtime = {
  report(request) {
    capturedRequest = request
    return { result: () => ({}) }
  },
}
const client = { writeReport: () => ({ result: () => ({}) }) }
const binding = new Overloaded(client, '0x0000000000000000000000000000000000000001')
const cases = [
  ['writeReportFromSetValue', [7n]],
  ['writeReportFromSetValue0', [1n << 200n]],
  ['writeReportFromSetValue1', ['hello']],
  ['writeReportFromSetValue2', [[{ count: 123n, label: 'nested tuple' }]]],
]
for (const [index, [method, args]] of cases.entries()) {
  capturedRequest = undefined
  binding[method](runtime, ...args)
  assert.ok(capturedRequest?.encodedPayload, method + ' must request a report')
  const fn = OverloadedABI[index]
  const expected = includeSelector
    ? encodeFunctionData({ abi: [fn], functionName: 'setValue', args })
    : encodeAbiParameters(fn.inputs, args)
  const actual = '0x' + Buffer.from(capturedRequest.encodedPayload, 'base64').toString('hex')
  assert.equal(actual, expected, method + ' must encode its own overload')
}
`
			cmd := exec.Command("node", "--input-type=module", "-e", script, compiledFile, fmt.Sprint(includeSelector))
			cmd.Dir = repoRoot
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, string(output))
		})
	}
}

func requireTSReportTestDependencies(t *testing.T, repoRoot string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for TypeScript report payload tests")
	}
	for _, name := range []string{"viem", "typescript"} {
		if _, err := os.Stat(filepath.Join(repoRoot, "node_modules", name)); err != nil {
			t.Skipf("node_modules/%s is required for TypeScript report payload tests", name)
		}
	}
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

func compileGeneratedTS(t *testing.T, repoRoot, tsFile string) string {
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
	return compiledFile
}

func generatedTSReportPayloadHex(t *testing.T, repoRoot, tsFile string) string {
	t.Helper()
	compiledFile := compileGeneratedTS(t, repoRoot, tsFile)

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
	cmd := exec.Command("node", "--input-type=module", "-e", script, compiledFile)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
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
