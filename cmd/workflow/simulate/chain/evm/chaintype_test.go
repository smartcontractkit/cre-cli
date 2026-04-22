package evm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"io"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func bigOne() *big.Int { return big.NewInt(1) }

func nopCommonLogger() logger.Logger {
	lg := logger.NewWithSync(io.Discard)
	return lg
}

func newRegistry(t *testing.T) *capabilities.Registry {
	t.Helper()
	r := capabilities.NewRegistry(logger.Test(t))
	return r
}

// stdioMu serialises os.Stderr / os.Stdout hijacks so parallel capture tests
// don't clobber each other's pipes.
var stdioMu sync.Mutex

// captureStderr captures anything written to os.Stderr during fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	stdioMu.Lock()
	defer stdioMu.Unlock()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	defer func() {
		os.Stderr = old
	}()

	fn()

	_ = w.Close()
	<-done
	return buf.String()
}

func newEVMChainType() *EVMChainType {
	lg := zerolog.Nop()
	return &EVMChainType{log: &lg}
}

// Valid anvil dev key #0; known non-sentinel.
const validPK = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

func TestEVMChainType_ResolveKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pk          string
		broadcast   bool
		wantErr     bool
		errContains string
		wantStderr  string // substring expected in ui.Warning stderr; "" = no warn
		checkD1     bool   // sentinel (D==1) expected if non-err non-broadcast
	}{
		{
			name:      "valid key, non-broadcast, returns parsed key, no warning",
			pk:        validPK,
			broadcast: false,
		},
		{
			name:      "valid key, broadcast, returns parsed key",
			pk:        validPK,
			broadcast: true,
		},
		{
			name:       "invalid hex, non-broadcast, falls back to sentinel and warns",
			pk:         "notahex",
			broadcast:  false,
			wantStderr: "Using default private key for chain write simulation",
			checkD1:    true,
		},
		{
			name:       "empty key, non-broadcast, falls back to sentinel and warns",
			pk:         "",
			broadcast:  false,
			wantStderr: "Using default private key for chain write simulation",
			checkD1:    true,
		},
		{
			name:       "0x-prefixed key (invalid per HexToECDSA), non-broadcast, falls back + warns",
			pk:         "0x" + validPK,
			broadcast:  false,
			wantStderr: "Using default private key",
			checkD1:    true,
		},
		{
			name:       "too-short key, non-broadcast, falls back + warns",
			pk:         "ab",
			broadcast:  false,
			wantStderr: "Using default private key",
			checkD1:    true,
		},
		{
			name:        "invalid hex, broadcast, hard error",
			pk:          "notahex",
			broadcast:   true,
			wantErr:     true,
			errContains: "failed to parse private key, required to broadcast",
		},
		{
			name:        "empty key, broadcast, hard error",
			pk:          "",
			broadcast:   true,
			wantErr:     true,
			errContains: "CRE_ETH_PRIVATE_KEY",
		},
		{
			name:        "sentinel key, broadcast, hard error about configuring valid key",
			pk:          defaultSentinelPrivateKey,
			broadcast:   true,
			wantErr:     true,
			errContains: "configure a valid private key",
		},
		{
			name:      "sentinel key, non-broadcast, returned without warning (parses fine)",
			pk:        defaultSentinelPrivateKey,
			broadcast: false,
			checkD1:   true,
		},
		{
			name:        "too-short key, broadcast, hard error",
			pk:          "ab",
			broadcast:   true,
			wantErr:     true,
			errContains: "required to broadcast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := newEVMChainType()
			s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: tt.pk}}

			var got interface{}
			var err error
			stderr := captureStderr(t, func() {
				got, err = ct.ResolveKey(s, tt.broadcast)
			})

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			pk, ok := got.(*ecdsa.PrivateKey)
			require.True(t, ok, "expected *ecdsa.PrivateKey, got %T", got)
			require.NotNil(t, pk)
			if tt.checkD1 {
				assert.Equal(t, 0, pk.D.Cmp(bigOne()), "expected sentinel D==1")
			}
			if tt.wantStderr == "" {
				assert.NotContains(t, stderr, "Using default private key",
					"did not expect sentinel warning but got: %s", stderr)
			} else {
				assert.Contains(t, stderr, tt.wantStderr)
			}
		})
	}
}

func TestEVMChainType_ResolveTriggerData_NoClient(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	_, err := ct.ResolveTriggerData(context.Background(), 777, chain.TriggerParams{
		Clients:     map[uint64]chain.ChainClient{},
		Interactive: false,
		ChainTypeInputs: map[string]string{
			"evm-tx-hash":     "0x" + strings.Repeat("a", 64),
			"evm-event-index": "0",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RPC configured for chain selector 777")
}

func TestEVMChainType_ResolveTriggerData_WrongClientType(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	_, err := ct.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{
		Clients:     map[uint64]chain.ChainClient{1: "not-an-ethclient"},
		Interactive: false,
		ChainTypeInputs: map[string]string{
			"evm-tx-hash":     "0x" + strings.Repeat("a", 64),
			"evm-event-index": "0",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for EVM chain selector 1")
}

func TestEVMChainType_ExecuteTrigger_NotRegistered(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	err := ct.ExecuteTrigger(context.Background(), 1, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EVM: capabilities not registered")
}

func TestEVMChainType_ExecuteTrigger_UnknownSelector(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	// set evmChains with empty map to bypass nil check
	ct.evmChains = &EVMChainCapabilities{EVMChains: nil}
	err := ct.ExecuteTrigger(context.Background(), 999, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no EVM chain initialized for selector 999")
}

func TestEVMChainType_Supports_WhenNotRegistered_ReturnsFalse(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	assert.False(t, ct.Supports(1))
	assert.False(t, ct.Supports(0))
}

func TestEVMChainType_RegisterCapabilities_WrongClientType(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	cfg := chain.CapabilityConfig{
		Clients:    map[uint64]chain.ChainClient{1: "not-an-ethclient"},
		Forwarders: map[uint64]string{1: "0x" + strings.Repeat("a", 40)},
	}
	_, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client for selector 1 is not *ethclient.Client")
}

// With no clients the caps should still construct, no type-assertion error.
func TestEVMChainType_RegisterCapabilities_NoClients_ConstructsEmpty(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	cfg := chain.CapabilityConfig{
		Clients:    map[uint64]chain.ChainClient{},
		Forwarders: map[uint64]string{},
		Logger:     nopCommonLogger(),
		Registry:   newRegistry(t),
	}
	srvcs, err := ct.RegisterCapabilities(context.Background(), cfg)
	// No clients means no chains; should succeed with empty service list.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Empty(t, srvcs)
	assert.False(t, ct.Supports(1))
}

func TestEVMChainType_RunHealthCheck_PropagatesInvalidClientType(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	resolved := chain.ResolvedChains{
		Clients: map[uint64]chain.ChainClient{1: "not-ethclient"},
	}
	err := ct.RunHealthCheck(resolved)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for EVM chain type")
}

func TestEVMChainType_RunHealthCheck_NoClients_Errors(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	err := ct.RunHealthCheck(chain.ResolvedChains{
		Clients: map[uint64]chain.ChainClient{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RPC URLs found")
}

func TestEVMChainType_RegisteredInFactoryRegistry(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	chain.Build(&lg)
	names := chain.Names()
	found := false
	for _, n := range names {
		if n == "evm" {
			found = true
			break
		}
	}
	require.True(t, found, "evm chain type should be registered at init; got %v", names)

	ct, err := chain.Get("evm")
	require.NoError(t, err)
	require.Equal(t, "evm", ct.Name())
}

func TestEVMChainType_CollectCLIInputs_BothSet(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	v := viper.New()
	v.Set("evm-tx-hash", "0xabc123")
	v.Set("evm-event-index", 2)

	result := ct.CollectCLIInputs(v)
	assert.Equal(t, "0xabc123", result[TriggerInputTxHash])
	assert.Equal(t, "2", result[TriggerInputEventIndex])
}

func TestEVMChainType_CollectCLIInputs_NegativeIndexOmitted(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	v := viper.New()
	v.Set("evm-tx-hash", "0xabc")
	v.Set("evm-event-index", -1)

	result := ct.CollectCLIInputs(v)
	assert.Equal(t, "0xabc", result[TriggerInputTxHash])
	_, hasIndex := result[TriggerInputEventIndex]
	assert.False(t, hasIndex, "negative index should be omitted")
}

func TestEVMChainType_CollectCLIInputs_EmptyTxHashOmitted(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	v := viper.New()
	v.Set("evm-tx-hash", "")
	v.Set("evm-event-index", 0)

	result := ct.CollectCLIInputs(v)
	_, hasTx := result[TriggerInputTxHash]
	assert.False(t, hasTx, "empty tx hash should be omitted")
	assert.Equal(t, "0", result[TriggerInputEventIndex])
}

func TestEVMChainType_CollectCLIInputs_DefaultsOnly(t *testing.T) {
	t.Parallel()
	ct := newEVMChainType()
	v := viper.New()
	// Viper defaults int to 0; simulate's flag registration sets default to -1.
	// Without explicit flag defaults, CollectCLIInputs sees 0 (>= 0) and includes it.
	v.SetDefault("evm-event-index", -1)

	result := ct.CollectCLIInputs(v)
	assert.Empty(t, result)
}
