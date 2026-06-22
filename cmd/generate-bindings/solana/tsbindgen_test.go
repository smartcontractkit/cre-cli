package solana

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestGenerateBindingsTS_WriteMethodCollision(t *testing.T) {
	idlPath := writeTestIdl(t, `[
		{"name": "Foo", "type": {"kind": "struct", "fields": [{"name": "x", "type": "u8"}]}},
		{"name": "Foos", "type": {"kind": "struct", "fields": [{"name": "y", "type": "u8"}]}}
	]`)
	_, err := GenerateBindingsTS(idlPath, "fail_loud", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write method name collision")
}
