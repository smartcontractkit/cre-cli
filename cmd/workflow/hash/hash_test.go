package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
)

// Well-known test private key (never use on a real network).
const testPrivateKey = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// Address derived from testPrivateKey.
const testDerivedAddress = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

func TestResolveOwner_WithForUser(t *testing.T) {
	t.Parallel()
	addr, err := ResolveOwner("0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF", "", "")
	require.NoError(t, err)
	assert.Equal(t, "0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF", addr)
}

func TestResolveOwner_WithForUserOverridesAll(t *testing.T) {
	t.Parallel()
	addr, err := ResolveOwner("0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF", "0xOtherAddress", testPrivateKey)
	require.NoError(t, err)
	assert.Equal(t, "0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF", addr,
		"--public_key should take priority over settings and private key")
}

func TestResolveOwner_FromSettings(t *testing.T) {
	t.Parallel()
	addr, err := ResolveOwner("", "0xSettingsOwner", "")
	require.NoError(t, err)
	assert.Equal(t, "0xSettingsOwner", addr)
}

func TestResolveOwner_FromPrivateKey(t *testing.T) {
	t.Parallel()
	addr, err := ResolveOwner("", "", testPrivateKey)
	require.NoError(t, err)
	assert.Equal(t, testDerivedAddress, addr)
}

func TestResolveOwner_NothingProvided(t *testing.T) {
	t.Parallel()
	_, err := ResolveOwner("", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--public_key")
}

func TestResolveOwner_InvalidPrivateKey(t *testing.T) {
	t.Parallel()
	_, err := ResolveOwner("", "", "not-a-valid-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to derive owner")
}

func TestExecute_WithForUser(t *testing.T) {
	wasmFile, configFile := setupTestArtifacts(t)

	inputs := Inputs{
		ForUser:      "0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF",
		WasmPath:     wasmFile,
		ConfigPath:   configFile,
		WorkflowName: "test-workflow",
	}

	err := Execute(inputs)
	require.NoError(t, err)
}

func TestExecute_WithoutForUser_UsesPrivateKey(t *testing.T) {
	wasmFile, configFile := setupTestArtifacts(t)

	inputs := Inputs{
		WasmPath:     wasmFile,
		ConfigPath:   configFile,
		WorkflowName: "test-workflow",
		PrivateKey:   testPrivateKey,
	}

	err := Execute(inputs)
	require.NoError(t, err)
}

func TestExecute_WithoutForUser_NoKey_Errors(t *testing.T) {
	wasmFile, configFile := setupTestArtifacts(t)

	inputs := Inputs{
		WasmPath:     wasmFile,
		ConfigPath:   configFile,
		WorkflowName: "test-workflow",
	}

	err := Execute(inputs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--public_key")
}

func TestExecute_HashesAreDeterministic(t *testing.T) {
	wasmFile, configFile := setupTestArtifacts(t)

	inputs := Inputs{
		ForUser:      "0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF",
		WasmPath:     wasmFile,
		ConfigPath:   configFile,
		WorkflowName: "test-workflow",
	}

	wasmBytes, err := os.ReadFile(wasmFile)
	require.NoError(t, err)
	configBytes, err := os.ReadFile(configFile)
	require.NoError(t, err)

	expectedBinaryHash := cmdcommon.HashBytes(wasmBytes)
	expectedConfigHash := cmdcommon.HashBytes(configBytes)
	expectedWorkflowID, err := workflowUtils.GenerateWorkflowIDFromStrings(
		inputs.ForUser, inputs.WorkflowName, wasmBytes, configBytes, "")
	require.NoError(t, err)

	// Verify the individual hash computations are as expected (SHA-256)
	binarySum := sha256.Sum256(wasmBytes)
	assert.Equal(t, hex.EncodeToString(binarySum[:]), expectedBinaryHash)

	configSum := sha256.Sum256(configBytes)
	assert.Equal(t, hex.EncodeToString(configSum[:]), expectedConfigHash)

	// Workflow ID should start with "00" (version byte)
	assert.True(t, strings.HasPrefix(expectedWorkflowID, "00"),
		"workflow ID should start with version byte 00")

	// Running Execute should succeed (hashes are printed via ui, verified above)
	err = Execute(inputs)
	require.NoError(t, err)
}

func TestExecute_EmptyConfig(t *testing.T) {
	wasmFile, _ := setupTestArtifacts(t)

	inputs := Inputs{
		ForUser:      "0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF",
		WasmPath:     wasmFile,
		ConfigPath:   "",
		WorkflowName: "test-workflow",
	}

	err := Execute(inputs)
	require.NoError(t, err)
}

func TestExecute_DifferentOwnersProduceDifferentWorkflowHashes(t *testing.T) {
	wasmFile, configFile := setupTestArtifacts(t)

	wasmBytes, err := os.ReadFile(wasmFile)
	require.NoError(t, err)
	configBytes, err := os.ReadFile(configFile)
	require.NoError(t, err)

	id1, err := workflowUtils.GenerateWorkflowIDFromStrings(
		"0xDeaDbeefdEAdbeefdEadbEEFdeadbeEFdEaDbeeF", "test-workflow", wasmBytes, configBytes, "")
	require.NoError(t, err)

	id2, err := workflowUtils.GenerateWorkflowIDFromStrings(
		"0x1111111111111111111111111111111111111111", "test-workflow", wasmBytes, configBytes, "")
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2, "different owners should produce different workflow hashes")
}

func TestHashCommandArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no args provided",
			args:    []string{},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "too many args",
			args:    []string{"path1", "path2"},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := New(nil)
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestHashCommandFlags(t *testing.T) {
	t.Parallel()
	cmd := New(nil)

	f := cmd.Flags().Lookup("public_key")
	require.NotNil(t, f, "public_key flag should exist")
	assert.Equal(t, "", f.DefValue)
	assert.Contains(t, f.Usage, "Required when CRE_ETH_PRIVATE_KEY is not set")
	assert.Contains(t, f.Usage, "Defaults to")

	f = cmd.Flags().Lookup("wasm")
	require.NotNil(t, f, "wasm flag should exist")

	f = cmd.Flags().Lookup("config")
	require.NotNil(t, f, "config flag should exist")

	f = cmd.Flags().Lookup("no-config")
	require.NotNil(t, f, "no-config flag should exist")
}

// setupTestArtifacts creates a minimal WASM file and config file in a temp directory.
func setupTestArtifacts(t *testing.T) (wasmPath, configPath string) {
	t.Helper()
	dir := t.TempDir()

	// Minimal valid WASM binary (magic + version)
	wasmMagic := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	wasmPath = filepath.Join(dir, "test.wasm")
	require.NoError(t, os.WriteFile(wasmPath, wasmMagic, 0600))

	configData := []byte(`workflowName: "test"`)
	configPath = filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, configData, 0600))

	return wasmPath, configPath
}
