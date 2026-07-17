package solana

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestIdlNoAddress writes a minimal valid IDL without an address field.
func writeTestIdlNoAddress(t *testing.T) string {
	t.Helper()
	idl := `{
  "metadata": {"name": "no_addr", "version": "0.1.0", "spec": "0.1.0"},
  "instructions": [
    {"name": "on_report", "discriminator": [214,173,18,221,173,148,151,208], "accounts": [], "args": []}
  ],
  "accounts": [],
  "events": [],
  "errors": [],
  "types": []
}`
	path := filepath.Join(t.TempDir(), "no_addr.json")
	require.NoError(t, os.WriteFile(path, []byte(idl), 0o600))
	return path
}

// writeTestIdl writes a minimal valid IDL with the given types JSON fragment
// and returns its path.
func writeTestIdl(t *testing.T, typesJSON string) string {
	t.Helper()
	idl := `{
  "address": "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
  "metadata": {"name": "fail_loud", "version": "0.1.0", "spec": "0.1.0"},
  "instructions": [
    {"name": "on_report", "discriminator": [214,173,18,221,173,148,151,208], "accounts": [], "args": []}
  ],
  "accounts": [],
  "events": [],
  "errors": [],
  "types": ` + typesJSON + `
}`
	path := filepath.Join(t.TempDir(), "fail_loud.json")
	require.NoError(t, os.WriteFile(path, []byte(idl), 0o600))
	return path
}

// TestGenerateBindingsTS_Golden regenerates the TypeScript bindings for the
// checked-in fixture IDLs and compares them byte-for-byte against the golden
// files. Regenerate goldens with `go generate ./cmd/generate-bindings/solana`.
func TestGenerateBindingsTS_Golden(t *testing.T) {
	cases := []struct {
		program   string
		className string
		goldenDir string
	}{
		{program: "data_storage", className: "DataStorage", goldenDir: "testdata/data_storage_ts"},
		{program: "feature_matrix", className: "FeatureMatrix", goldenDir: "testdata/feature_matrix_ts"},
	}

	for _, tc := range cases {
		t.Run(tc.program, func(t *testing.T) {
			outDir := t.TempDir()
			className, err := GenerateBindingsTS(
				filepath.Join("testdata", "contracts", "idl", tc.program+".json"),
				tc.program,
				outDir,
			)
			require.NoError(t, err)
			assert.Equal(t, tc.className, className)

			for _, file := range []string{tc.className + ".ts", tc.className + "_mock.ts"} {
				generated, err := os.ReadFile(filepath.Join(outDir, file))
				require.NoError(t, err)
				golden, err := os.ReadFile(filepath.Join(tc.goldenDir, file))
				require.NoError(t, err)
				assert.Equal(t, string(golden), string(generated),
					"%s differs from golden; regenerate with `go generate ./cmd/generate-bindings/solana` if the change is intended", file)
			}
		})
	}
}

func TestGenerateBindingsTS_GeneratedContent(t *testing.T) {
	outDir := t.TempDir()
	_, err := GenerateBindingsTS(
		filepath.Join("testdata", "contracts", "idl", "feature_matrix.json"),
		"feature_matrix",
		outDir,
	)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outDir, "FeatureMatrix.ts"))
	require.NoError(t, err)
	source := string(content)

	// Spot-check the v1 type-coverage matrix mappings.
	assert.Contains(t, source, "export enum Color {")
	assert.Contains(t, source, "getEnumCodec(Color)")
	assert.Contains(t, source, "huge: bigint")
	assert.Contains(t, source, "getU128Codec()")
	assert.Contains(t, source, "owner: Address")
	assert.Contains(t, source, "getAddressCodec()")
	assert.Contains(t, source, "maybeNote: string | null")
	assert.Contains(t, source, "getNullableCodec(addCodecSizePrefix(getUtf8Codec(), getU32Codec()))")
	assert.Contains(t, source, "getArrayCodec(getU8Codec(), { size: 32 })")
	assert.Contains(t, source, "['nested', innerCodec],")
	assert.Contains(t, source, "maybeNested: Inner | null")
	assert.Contains(t, source, "colorHistory: Color[]")
	// JS reserved word field gets an underscore suffix.
	assert.Contains(t, source, "default_: boolean")
	// Per-struct write methods (single and slice).
	assert.Contains(t, source, "writeReportFromEverything(")
	assert.Contains(t, source, "writeReportFromEverythings(")
	assert.Contains(t, source, "writeReportFromBorshEncodedVec(")
	// No dispatch functions for feature_matrix (no accounts or events in IDL).
	assert.NotContains(t, source, "parseAnyAccount")
	assert.NotContains(t, source, "parseAnyEvent")
}

func TestGenerateBindingsTS_DispatchFunctions(t *testing.T) {
	outDir := t.TempDir()
	_, err := GenerateBindingsTS(
		filepath.Join("testdata", "contracts", "idl", "data_storage.json"),
		"data_storage",
		outDir,
	)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outDir, "DataStorage.ts"))
	require.NoError(t, err)
	source := string(content)

	assert.Contains(t, source, "export const parseAnyAccount = (data: Uint8Array): DataAccount =>")
	assert.Contains(t, source, "export const parseAnyEvent = (data: Uint8Array): AccessLogged | DynamicEvent | NoFields =>")
	assert.Contains(t, source, "if (matches(ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR)) return decodeDataAccountAccount(data)")
	assert.Contains(t, source, "if (matches(EVENT_ACCESS_LOGGED_DISCRIMINATOR)) return decodeAccessLoggedEvent(data)")
	assert.Contains(t, source, "unknown account discriminator")
	assert.Contains(t, source, "unknown event discriminator")
}

func TestGenerateBindingsTS_LogTriggers(t *testing.T) {
	outDir := t.TempDir()
	_, err := GenerateBindingsTS(
		filepath.Join("testdata", "contracts", "idl", "data_storage.json"),
		"data_storage",
		outDir,
	)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outDir, "DataStorage.ts"))
	require.NoError(t, err)
	source := string(content)

	// Per-event filters type, subkey encoder, and typed trigger method.
	assert.Contains(t, source, "export type AccessLoggedFilters = {")
	assert.Contains(t, source, "caller?: Address | null")
	assert.Contains(t, source, "export const encodeAccessLoggedSubkeys = (filters: AccessLoggedFilters[]): SolanaSubkeyConfigJson[] =>")
	assert.Contains(t, source, "logTriggerAccessLoggedLog(")
	assert.Contains(t, source, "): Trigger<SolanaLog, SolanaDecodedLog<AccessLogged>> {")
	// Subkey paths use the Go bindings' PascalCase names.
	assert.Contains(t, source, "subkeys.push({ path: ['Caller'], comparers: callerComparers })")
	// Pubkey filter values are converted from base58 before encoding.
	assert.Contains(t, source, "value: bytesToBase64(solanaAddressToBytes(f.caller))")
	// The compact IDL is embedded once as base64 and sent as contractIdlJson.
	assert.Contains(t, source, "const DATA_STORAGE_IDL_BASE64 = '")
	assert.Contains(t, source, "contractIdlJson: DATA_STORAGE_IDL_BASE64,")
	// CPI opt-in wires the anchor:event self-CPI filter.
	assert.Contains(t, source, "config.cpiFilterConfig = anchorCPILogTriggerConfig(this.programId)")
	// The decoded data rides along with the raw log.
	assert.Contains(t, source, "data: decodeAccessLoggedEvent(log.data),")
	// Non-filterable fields (vec, defined) are excluded from DynamicEvent filters.
	assert.NotContains(t, source, "userData?:")
	assert.NotContains(t, source, "metadataArray?:")
	// An event with no filterable fields still gets a trigger with empty filters.
	assert.Contains(t, source, "export type NoFieldsFilters = Record<string, never>")
	assert.Contains(t, source, "export const encodeNoFieldsSubkeys = (_filters: NoFieldsFilters[]): SolanaSubkeyConfigJson[] => []")
	assert.Contains(t, source, "logTriggerNoFieldsLog(")

	// An IDL without events must not emit trigger code or its imports.
	fmOut := t.TempDir()
	_, err = GenerateBindingsTS(
		filepath.Join("testdata", "contracts", "idl", "feature_matrix.json"),
		"feature_matrix",
		fmOut,
	)
	require.NoError(t, err)
	fmContent, err := os.ReadFile(filepath.Join(fmOut, "FeatureMatrix.ts"))
	require.NoError(t, err)
	assert.NotContains(t, string(fmContent), "logTrigger")
	assert.NotContains(t, string(fmContent), "anchorCPILogTriggerConfig")
	assert.NotContains(t, string(fmContent), "prepareSubkeyValue")
}

func TestGenerateBindingsTS_TriggerFilterEncodings(t *testing.T) {
	idl := `{
  "address": "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
  "metadata": {"name": "encodings", "version": "0.1.0", "spec": "0.1.0"},
  "instructions": [
    {"name": "on_report", "discriminator": [214,173,18,221,173,148,151,208], "accounts": [], "args": []}
  ],
  "accounts": [],
  "events": [{"name": "Mixed", "discriminator": [1,2,3,4,5,6,7,8]}],
  "errors": [],
  "types": [{"name": "Mixed", "type": {"kind": "struct", "fields": [
    {"name": "small", "type": "u8"},
    {"name": "amount", "type": "u64"},
    {"name": "ratio", "type": "f64"},
    {"name": "maybe_tag", "type": {"option": "string"}},
    {"name": "flag", "type": "bool"},
    {"name": "huge", "type": "u128"},
    {"name": "blob", "type": "bytes"},
    {"name": "list", "type": {"vec": "u8"}},
    {"name": "fixed", "type": {"array": ["u8", 32]}}
  ]}}]
}`
	idlPath := filepath.Join(t.TempDir(), "encodings.json")
	require.NoError(t, os.WriteFile(idlPath, []byte(idl), 0o600))

	outDir := t.TempDir()
	_, err := GenerateBindingsTS(idlPath, "encodings", outDir)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outDir, "Encodings.ts"))
	require.NoError(t, err)
	source := string(content)

	// Scalar filter fields with their subkey encoders.
	assert.Contains(t, source, "small?: number | null")
	assert.Contains(t, source, "amount?: bigint | null")
	assert.Contains(t, source, "ratio?: number | null")
	assert.Contains(t, source, "blob?: Uint8Array | null")
	// Option is unwrapped for the filter value type.
	assert.Contains(t, source, "maybeTag?: string | null")
	assert.Contains(t, source, "value: bytesToBase64(prepareSubkeyValue(f.amount))")
	assert.Contains(t, source, "value: bytesToBase64(prepareSubkeyFloatValue(f.ratio))")
	assert.Contains(t, source, "prepareSubkeyFloatValue,")
	// Subkey paths use the Go bindings' PascalCase names.
	assert.Contains(t, source, "path: ['MaybeTag']")
	// bool, u128, vec, and fixed arrays are not auto-filterable.
	assert.NotContains(t, source, "flag?:")
	assert.NotContains(t, source, "huge?:")
	assert.NotContains(t, source, "list?:")
	assert.NotContains(t, source, "fixed?:")
}

func TestGenerateBindingsTS_FailLoud(t *testing.T) {
	cases := []struct {
		name      string
		typesJSON string
		wantErr   string
	}{
		{
			name:      "u256",
			typesJSON: `[{"name": "Big", "type": {"kind": "struct", "fields": [{"name": "x", "type": "u256"}]}}]`,
			wantErr:   "u256",
		},
		{
			name:      "i256",
			typesJSON: `[{"name": "Big", "type": {"kind": "struct", "fields": [{"name": "x", "type": "i256"}]}}]`,
			wantErr:   "i256",
		},
		{
			name:      "coption",
			typesJSON: `[{"name": "Opt", "type": {"kind": "struct", "fields": [{"name": "x", "type": {"coption": "u64"}}]}}]`,
			wantErr:   "COption",
		},
		{
			name: "data-carrying enum",
			typesJSON: `[{"name": "Shape", "type": {"kind": "enum", "variants": [
				{"name": "Circle", "fields": [{"name": "radius", "type": "u64"}]},
				{"name": "Point"}
			]}}]`,
			wantErr: "data-carrying enums",
		},
		{
			name:      "tuple struct",
			typesJSON: `[{"name": "Pair", "type": {"kind": "struct", "fields": ["u64", "string"]}}]`,
			wantErr:   "tuple struct fields",
		},
		{
			name: "cyclic types",
			typesJSON: `[
				{"name": "A", "type": {"kind": "struct", "fields": [{"name": "b", "type": {"defined": {"name": "B"}}}]}},
				{"name": "B", "type": {"kind": "struct", "fields": [{"name": "a", "type": {"defined": {"name": "A"}}}]}}
			]`,
			wantErr: "cyclic type reference",
		},
		{
			name: "field name collision after camelCase",
			typesJSON: `[{"name": "Collide", "type": {"kind": "struct", "fields": [
				{"name": "user_data", "type": "u8"},
				{"name": "userData", "type": "u8"}
			]}}]`,
			wantErr: "both map to",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idlPath := writeTestIdl(t, tc.typesJSON)
			_, err := GenerateBindingsTS(idlPath, "fail_loud", t.TempDir())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestGenerateBindingsTS_MissingAddress verifies that an IDL without an address
// field succeeds (previously it returned an error). The generated program ID
// constant must be empty so callers know to supply the address at construction time.
func TestGenerateBindingsTS_MissingAddress(t *testing.T) {
	idlPath := writeTestIdlNoAddress(t)
	outDir := t.TempDir()
	className, err := GenerateBindingsTS(idlPath, "no_addr", outDir)
	require.NoError(t, err)
	assert.Equal(t, "NoAddr", className)

	content, err := os.ReadFile(filepath.Join(outDir, "NoAddr.ts"))
	require.NoError(t, err)
	source := string(content)

	// Program ID constant must be empty — caller must supply it at construction time.
	assert.Contains(t, source, "export const NO_ADDR_PROGRAM_ID = ''")
}

func TestGenerateBindingsTS_WriteMethodCollision(t *testing.T) {
	idlPath := writeTestIdl(t, `[
		{"name": "Foo", "type": {"kind": "struct", "fields": [{"name": "x", "type": "u8"}]}},
		{"name": "Foos", "type": {"kind": "struct", "fields": [{"name": "y", "type": "u8"}]}}
	]`)
	_, err := GenerateBindingsTS(idlPath, "fail_loud", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write method name collision")
}
