package solana

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	solanafakes "github.com/smartcontractkit/chainlink-solana/contracts/capabilities/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func newSolanaChainType() *SolanaChainType {
	lg := zerolog.Nop()
	return &SolanaChainType{log: &lg}
}

func newTestFakeSolanaChain(t *testing.T, selector uint64) *solanafakes.FakeSolanaChain {
	t.Helper()
	fc, err := solanafakes.NewFakeSolanaChain(
		logger.Test(t),
		solanarpc.New("http://localhost:1"),
		solana.NewWallet().PrivateKey,
		solana.NewWallet().PublicKey(),
		solana.NewWallet().PublicKey(),
		selector,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, fc)
	return fc
}

func TestSolanaChainType_ResolveTriggerData_NoClient(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	_, err := ct.ResolveTriggerData(context.Background(), 777, chain.TriggerParams{
		Clients: map[uint64]chain.ChainClient{},
		ChainTypeInputs: map[string]string{
			TriggerInputTxSig:      "3sig",
			TriggerInputEventIndex: "0",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RPC configured for chain selector 777")
}

func TestSolanaChainType_ResolveTriggerData_WrongClientType(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	_, err := ct.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{
		Clients: map[uint64]chain.ChainClient{1: "not-a-solana-client"},
		ChainTypeInputs: map[string]string{
			TriggerInputTxSig:      "3sig",
			TriggerInputEventIndex: "0",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for Solana chain selector 1")
}

func TestSolanaChainType_ResolveTriggerData_MissingInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		chainTypeInputs map[string]string
		errContains     string
	}{
		{
			name:            "missing tx sig",
			chainTypeInputs: map[string]string{TriggerInputEventIndex: "0"},
			errContains:     "--solana-tx-sig and --solana-event-index are required",
		},
		{
			name:            "missing event index",
			chainTypeInputs: map[string]string{TriggerInputTxSig: "3sig"},
			errContains:     "--solana-tx-sig and --solana-event-index are required",
		},
		{
			name: "invalid event index",
			chainTypeInputs: map[string]string{
				TriggerInputTxSig:      "3sig",
				TriggerInputEventIndex: "not-a-number",
			},
			errContains: "invalid --solana-event-index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ct := newSolanaChainType()
			_, err := ct.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{
				Clients:         map[uint64]chain.ChainClient{1: solanarpc.New("http://localhost:1")},
				ChainTypeInputs: tt.chainTypeInputs,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestSolanaChainType_ExecuteTrigger_NotRegistered(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	err := ct.ExecuteTrigger(context.Background(), 1, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "solana: capabilities not registered")
}

func TestSolanaChainType_ExecuteTrigger_UnknownSelector(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	ct.solanaChains = &SolanaChainCapabilities{SolanaChains: nil}
	err := ct.ExecuteTrigger(context.Background(), 999, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Solana chain initialized for selector 999")
}

func TestSolanaChainType_ExecuteTrigger_WrongTriggerDataType(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	fc := newTestFakeSolanaChain(t, 1)
	ct.solanaChains = &SolanaChainCapabilities{SolanaChains: map[uint64]*solanafakes.FakeSolanaChain{1: fc}}

	err := ct.ExecuteTrigger(context.Background(), 1, "regID", "not-a-solana-log")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "solana: trigger data is not *solana.Log")
}

func TestSolanaChainType_ExecuteTrigger_DeliversRegisteredLog(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	fc := newTestFakeSolanaChain(t, 1)
	ct.solanaChains = &SolanaChainCapabilities{SolanaChains: map[uint64]*solanafakes.FakeSolanaChain{1: fc}}

	prog := solana.NewWallet().PublicKey()
	ch, cerr := fc.RegisterLogTrigger(context.Background(), "regID", commonCap.RequestMetadata{}, &solcap.FilterLogTriggerRequest{Address: prog.Bytes()})
	require.Nil(t, cerr)

	err := ct.ExecuteTrigger(context.Background(), 1, "regID", &solcap.Log{Address: prog.Bytes()})
	require.NoError(t, err)

	select {
	case got := <-ch:
		assert.Equal(t, prog.Bytes(), got.Trigger.GetAddress())
	case <-time.After(2 * time.Second):
		t.Fatal("expected trigger event, got none")
	}
}

func TestSolanaChainType_Supports_WhenNotRegistered_ReturnsFalse(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	assert.False(t, ct.Supports(1))
	assert.False(t, ct.Supports(0))
}

func TestSolanaChainType_RegisteredInFactoryRegistry(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	chain.Build(&lg)
	names := chain.Names()
	found := false
	for _, n := range names {
		if n == "solana" {
			found = true
			break
		}
	}
	require.True(t, found, "solana chain type should be registered at init; got %v", names)

	ct, err := chain.Get("solana")
	require.NoError(t, err)
	require.Equal(t, "solana", ct.Name())
}

func TestSolanaChainType_CollectCLIInputs_BothSet(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	v := viper.New()
	v.Set(TriggerInputTxSig, "3sig")
	v.Set(TriggerInputEventIndex, 2)

	result := ct.CollectCLIInputs(v)
	assert.Equal(t, "3sig", result[TriggerInputTxSig])
	assert.Equal(t, "2", result[TriggerInputEventIndex])
}

func TestSolanaChainType_CollectCLIInputs_NegativeIndexOmitted(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	v := viper.New()
	v.Set(TriggerInputTxSig, "3sig")
	v.Set(TriggerInputEventIndex, -1)

	result := ct.CollectCLIInputs(v)
	assert.Equal(t, "3sig", result[TriggerInputTxSig])
	_, hasIndex := result[TriggerInputEventIndex]
	assert.False(t, hasIndex, "negative index should be omitted")
}

func TestSolanaChainType_CollectCLIInputs_EmptyTxSigOmitted(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	v := viper.New()
	v.Set(TriggerInputTxSig, "")
	v.Set(TriggerInputEventIndex, 0)

	result := ct.CollectCLIInputs(v)
	_, hasSig := result[TriggerInputTxSig]
	assert.False(t, hasSig, "empty tx sig should be omitted")
	assert.Equal(t, "0", result[TriggerInputEventIndex])
}

func TestSolanaChainType_CollectCLIInputs_DefaultsOnly(t *testing.T) {
	t.Parallel()
	ct := newSolanaChainType()
	v := viper.New()
	v.SetDefault(TriggerInputEventIndex, -1)

	result := ct.CollectCLIInputs(v)
	assert.Empty(t, result)
}

// The ~ form is the one we tell users to configure, so cover it explicitly.
func TestSolanaChainType_ResolveKey_TildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	key, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dir, err := os.MkdirTemp(home, "cre-solana-key-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	keyfile := writeKeygenFile(t, dir, "id.json", key)
	rel, err := filepath.Rel(home, keyfile)
	require.NoError(t, err)

	s := &settings.Settings{User: settings.UserSettings{
		PrivateKeys: map[string]string{settings.Solana.Name: filepath.Join("~", rel)},
	}}

	got, err := newSolanaChainType().ResolveKey(s, false)
	require.NoError(t, err)
	pk, ok := got.(solana.PrivateKey)
	require.True(t, ok, "expected solana.PrivateKey, got %T", got)
	assert.Equal(t, key.PublicKey(), pk.PublicKey())
}

// writeKeygenFile writes key in the JSON byte-array form that `solana-keygen`
// produces and returns the file path.
func writeKeygenFile(t *testing.T, dir, name string, key []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, keygenJSON(t, key), 0o600))
	return path
}

// keygenJSON renders key as the JSON array of numbers that `solana-keygen`
// writes. json.Marshal on a []byte would emit a base64 string instead, which
// is not the format users actually have on disk.
func keygenJSON(t *testing.T, key []byte) []byte {
	t.Helper()
	nums := make([]uint16, len(key))
	for i, b := range key {
		nums[i] = uint16(b)
	}
	b, err := json.Marshal(nums)
	require.NoError(t, err)
	return b
}

func TestSolanaChainType_ResolveKey(t *testing.T) {
	key, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dir := t.TempDir()
	keyfile := writeKeygenFile(t, dir, "id.json", key)
	shortFile := writeKeygenFile(t, dir, "short.json", key[:32])

	inline := keygenJSON(t, key)

	tests := []struct {
		name        string
		pk          string
		broadcast   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "base58 keypair, non-broadcast",
			pk:   key.String(),
		},
		{
			name:      "base58 keypair, broadcast",
			pk:        key.String(),
			broadcast: true,
		},
		{
			name: "path to solana-keygen keyfile",
			pk:   keyfile,
		},
		{
			name:      "path to solana-keygen keyfile, broadcast",
			pk:        keyfile,
			broadcast: true,
		},
		{
			name: "inline keyfile contents",
			pk:   string(inline),
		},
		{
			name:        "empty",
			pk:          "",
			wantErr:     true,
			errContains: "CRE_SOLANA_PRIVATE_KEY is required",
		},
		{
			name:        "empty suggests the keyfile path",
			pk:          "   ",
			wantErr:     true,
			errContains: "~/.config/solana/id.json",
		},
		{
			name:        "garbage",
			pk:          "not-a-key",
			wantErr:     true,
			errContains: "must be a base58-encoded 64-byte keypair",
		},
		{
			name:        "nonexistent path",
			pk:          filepath.Join(dir, "missing.json"),
			wantErr:     true,
			errContains: "must be a base58-encoded 64-byte keypair",
		},
		{
			name:        "keyfile with 32 bytes instead of 64",
			pk:          shortFile,
			wantErr:     true,
			errContains: "invalid private key length 32",
		},
		{
			name:        "inline contents with 32 bytes instead of 64",
			pk:          "[1,2,3]",
			wantErr:     true,
			errContains: "invalid private key length 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := newSolanaChainType()
			s := &settings.Settings{User: settings.UserSettings{
				PrivateKeys: map[string]string{settings.Solana.Name: tt.pk},
			}}

			got, err := ct.ResolveKey(s, tt.broadcast)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			pk, ok := got.(solana.PrivateKey)
			require.True(t, ok, "expected solana.PrivateKey, got %T", got)
			assert.Equal(t, key.PublicKey(), pk.PublicKey())
		})
	}
}
