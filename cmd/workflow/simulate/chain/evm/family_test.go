package evm

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"errors"
	"io"
	"math/big"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
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

// --- helpers ---

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

// captureStdout captures anything written to os.Stdout during fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdioMu.Lock()
	defer stdioMu.Unlock()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	defer func() {
		os.Stdout = old
	}()

	fn()

	_ = w.Close()
	<-done
	return buf.String()
}

func newFamily() *EVMFamily {
	lg := zerolog.Nop()
	return &EVMFamily{log: &lg}
}

// Valid anvil dev key #0; known non-sentinel.
const validPK = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

// ---------------------------------------------------------------------------
// Name
// ---------------------------------------------------------------------------

func TestEVMFamily_Name_IsEVM(t *testing.T) {
	t.Parallel()
	require.Equal(t, "evm", newFamily().Name())
}

// ---------------------------------------------------------------------------
// SupportedChains pass-through
// ---------------------------------------------------------------------------

func TestEVMFamily_SupportedChains_ReturnsPackageVar(t *testing.T) {
	t.Parallel()
	got := newFamily().SupportedChains()
	require.Equal(t, len(SupportedChains), len(got))
	require.Greater(t, len(got), 20, "expected many supported chains")
}

// ---------------------------------------------------------------------------
// ResolveKey table
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveKey(t *testing.T) {
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			f := newFamily()
			s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: tt.pk}}

			var got interface{}
			var err error
			stderr := captureStderr(t, func() {
				got, err = f.ResolveKey(s, tt.broadcast)
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

// ---------------------------------------------------------------------------
// ResolveKey sentinel identity
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveKey_SentinelDecodesToD1(t *testing.T) {
	t.Parallel()
	pk, err := crypto.HexToECDSA(defaultSentinelPrivateKey)
	require.NoError(t, err)
	require.Equal(t, 0, pk.D.Cmp(bigOne()))
}

// ---------------------------------------------------------------------------
// ResolveTriggerData — non-interactive validation
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveTriggerData_NoClient(t *testing.T) {
	t.Parallel()
	f := newFamily()
	_, err := f.ResolveTriggerData(context.Background(), 777, chain.TriggerParams{
		Clients:       map[uint64]chain.ChainClient{},
		Interactive:   false,
		EVMTxHash:     "0x" + strings.Repeat("a", 64),
		EVMEventIndex: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RPC configured for chain selector 777")
}

func TestEVMFamily_ResolveTriggerData_WrongClientType(t *testing.T) {
	t.Parallel()
	f := newFamily()
	_, err := f.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{
		Clients:       map[uint64]chain.ChainClient{1: "not-an-ethclient"},
		Interactive:   false,
		EVMTxHash:     "0x" + strings.Repeat("a", 64),
		EVMEventIndex: 0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for EVM chain selector 1")
}

// ---------------------------------------------------------------------------
// ExecuteTrigger
// ---------------------------------------------------------------------------

func TestEVMFamily_ExecuteTrigger_NotRegistered(t *testing.T) {
	t.Parallel()
	f := newFamily()
	err := f.ExecuteTrigger(context.Background(), 1, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EVM family: capabilities not registered")
}

func TestEVMFamily_ExecuteTrigger_UnknownSelector(t *testing.T) {
	t.Parallel()
	f := newFamily()
	// set evmChains with empty map to bypass nil check
	f.evmChains = &EVMChainCapabilities{EVMChains: nil}
	err := f.ExecuteTrigger(context.Background(), 999, "regID", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no EVM chain initialized for selector 999")
}

// ---------------------------------------------------------------------------
// HasSelector
// ---------------------------------------------------------------------------

func TestEVMFamily_HasSelector_WhenNotRegistered_ReturnsFalse(t *testing.T) {
	t.Parallel()
	f := newFamily()
	assert.False(t, f.HasSelector(1))
	assert.False(t, f.HasSelector(0))
}

func TestEVMFamily_HasSelector_EmptyMap_ReturnsFalse(t *testing.T) {
	t.Parallel()
	f := newFamily()
	f.evmChains = &EVMChainCapabilities{EVMChains: nil}
	assert.False(t, f.HasSelector(1))
}

// ---------------------------------------------------------------------------
// ParseTriggerChainSelector (via family interface)
// ---------------------------------------------------------------------------

func TestEVMFamily_ParseTriggerChainSelector_Delegates(t *testing.T) {
	t.Parallel()
	f := newFamily()
	got, ok := f.ParseTriggerChainSelector("evm:ChainSelector:42@1.0.0")
	require.True(t, ok)
	require.Equal(t, uint64(42), got)

	got, ok = f.ParseTriggerChainSelector("no-selector-here")
	require.False(t, ok)
	require.Zero(t, got)
}

// ---------------------------------------------------------------------------
// RegisterCapabilities type-assertion failures
// ---------------------------------------------------------------------------

func TestEVMFamily_RegisterCapabilities_WrongClientType(t *testing.T) {
	t.Parallel()
	f := newFamily()
	cfg := chain.CapabilityConfig{
		Clients:  map[uint64]chain.ChainClient{1: "not-an-ethclient"},
		Forwarders: map[uint64]string{1: "0x" + strings.Repeat("a", 40)},
	}
	_, err := f.RegisterCapabilities(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client for selector 1 is not *ethclient.Client")
}

// With no clients the caps should still construct, no type-assertion error.
func TestEVMFamily_RegisterCapabilities_NoClients_ConstructsEmpty(t *testing.T) {
	t.Parallel()
	f := newFamily()
	cfg := chain.CapabilityConfig{
		Clients:    map[uint64]chain.ChainClient{},
		Forwarders: map[uint64]string{},
		Logger:     nopCommonLogger(),
		Registry:   newRegistry(t),
	}
	srvcs, err := f.RegisterCapabilities(context.Background(), cfg)
	// No clients means no chains; should succeed with empty service list.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assert.Empty(t, srvcs)
	assert.False(t, f.HasSelector(1))
}

// ---------------------------------------------------------------------------
// RunHealthCheck plumbing
// ---------------------------------------------------------------------------

func TestEVMFamily_RunHealthCheck_PropagatesInvalidClientType(t *testing.T) {
	t.Parallel()
	f := newFamily()
	err := f.RunHealthCheck(map[uint64]chain.ChainClient{1: "not-ethclient"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for EVM family")
}

func TestEVMFamily_RunHealthCheck_NoClients_Errors(t *testing.T) {
	t.Parallel()
	f := newFamily()
	err := f.RunHealthCheck(map[uint64]chain.ChainClient{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RPC URLs found")
}

// ---------------------------------------------------------------------------
// ChainFamily interface contract
// ---------------------------------------------------------------------------

func TestEVMFamily_ImplementsChainFamily(t *testing.T) {
	t.Parallel()
	var _ chain.ChainFamily = (*EVMFamily)(nil)
}

// ---------------------------------------------------------------------------
// Registered via init
// ---------------------------------------------------------------------------

func TestEVMFamily_RegisteredInFactoryRegistry(t *testing.T) {
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
	require.True(t, found, "evm family should be registered at init; got %v", names)

	fam, err := chain.Get("evm")
	require.NoError(t, err)
	require.Equal(t, "evm", fam.Name())
}

// ---------------------------------------------------------------------------
// Sentinel error wrapping
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveKey_BroadcastErrorWrapsUnderlying(t *testing.T) {
	t.Parallel()
	f := newFamily()
	s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: "zz"}}
	_, err := f.ResolveKey(s, true)
	require.Error(t, err)
	// Must mention env var for operator-facing clarity.
	assert.Contains(t, err.Error(), "CRE_ETH_PRIVATE_KEY")
}

// ---------------------------------------------------------------------------
// Non-broadcast with valid key: no UI warning leaked
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveKey_ValidNonBroadcast_NoWarning(t *testing.T) {
	t.Parallel()
	f := newFamily()
	s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: validPK}}
	stderr := captureStderr(t, func() {
		_, err := f.ResolveKey(s, false)
		require.NoError(t, err)
	})
	assert.NotContains(t, stderr, "Using default private key")
}

// ---------------------------------------------------------------------------
// ExecuteTrigger wrong triggerData type
// ---------------------------------------------------------------------------

func TestEVMFamily_ExecuteTrigger_WrongTriggerDataType(t *testing.T) {
	t.Parallel()
	// Register a nil FakeEVMChain entry via map so the nil-check passes but the
	// triggerData type assertion fails first.
	f := newFamily()
	f.evmChains = &EVMChainCapabilities{EVMChains: nil}
	err := f.ExecuteTrigger(context.Background(), 1, "regID", "not-a-log")
	require.Error(t, err)
	// Whichever check fails first — both are acceptable.
	if !errorContainsAny(err, "trigger data is not *evm.Log", "no EVM chain initialized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func errorContainsAny(err error, subs ...string) bool {
	if err == nil {
		return false
	}
	for _, s := range subs {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}
	return false
}

// Defensive check: crypto.HexToECDSA rejects the string "0x..." so our
// fallback behaviour under non-broadcast keeps functioning even if a user
// copies their key with a prefix.
func TestEVMFamily_ResolveKey_PrefixedHex_FallsBackToSentinel(t *testing.T) {
	t.Parallel()
	f := newFamily()
	s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: "0x" + validPK}}
	stderr := captureStderr(t, func() {
		got, err := f.ResolveKey(s, false)
		require.NoError(t, err)
		pk := got.(*ecdsa.PrivateKey)
		require.Equal(t, 0, pk.D.Cmp(bigOne()))
	})
	assert.Contains(t, stderr, "Using default private key")
}

// ---------------------------------------------------------------------------
// Error type is standard error (not a sentinel) — ensures errors.Is behaviour.
// ---------------------------------------------------------------------------

func TestEVMFamily_ResolveKey_BroadcastError_IsError(t *testing.T) {
	t.Parallel()
	f := newFamily()
	s := &settings.Settings{User: settings.UserSettings{EthPrivateKey: ""}}
	_, err := f.ResolveKey(s, true)
	require.Error(t, err)
	require.NotNil(t, errors.Unwrap(err))
}
