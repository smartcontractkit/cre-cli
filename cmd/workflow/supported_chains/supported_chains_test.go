package supported_chains_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	supportedchains "github.com/smartcontractkit/cre-cli/cmd/workflow/supported_chains"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// captureStdout runs fn while redirecting os.Stdout to a buffer (ui package prints to stdout).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	var buf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()

	fn()
	require.NoError(t, w.Close())
	os.Stdout = old
	wg.Wait()
	_ = r.Close()
	return buf.String()
}

func TestSupportedChains_MissingTenantContext(t *testing.T) {
	t.Parallel()
	logger := zerolog.New(io.Discard)
	cmd := supportedchains.New(&runtime.Context{Logger: &logger})
	cmd.SetArgs([]string{})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "user context not available")
}

func TestSupportedChains_NilRuntimeContext(t *testing.T) {
	t.Parallel()
	cmd := supportedchains.New(nil)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestSupportedChains_EmptyForwarders(t *testing.T) {
	logger := zerolog.New(io.Discard)
	cmd := supportedchains.New(&runtime.Context{
		Logger: &logger,
		TenantContext: &tenantctx.EnvironmentContext{
			TenantID:   "t1",
			Forwarders: []tenantctx.Forwarder{},
		},
	})
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		require.NoError(t, cmd.Execute())
	})
	require.Contains(t, out, "No forwarders returned")
}

func TestSupportedChains_JSON(t *testing.T) {
	logger := zerolog.New(io.Discard)
	const sepoliaSel = uint64(16015286601757825753)
	cmd := supportedchains.New(&runtime.Context{
		Logger: &logger,
		TenantContext: &tenantctx.EnvironmentContext{
			Forwarders: []tenantctx.Forwarder{
				{ChainSelector: sepoliaSel, Address: "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"},
			},
		},
	})
	cmd.SetArgs([]string{"--output", "json"})

	out := captureStdout(t, func() {
		require.NoError(t, cmd.Execute())
	})
	var rows []supportedchains.ChainForwarderRow
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &rows))
	require.Len(t, rows, 1)
	require.Equal(t, sepoliaSel, rows[0].ChainSelector)
	require.Equal(t, "0x15fC6ae953E024d975e77382eEeC56A9101f9F88", rows[0].Address)
	require.NotEmpty(t, rows[0].ChainName)
	require.NotEqual(t, "-", rows[0].ChainName)
}

func TestSupportedChains_InvalidOutputFormat(t *testing.T) {
	t.Parallel()
	logger := zerolog.New(io.Discard)
	cmd := supportedchains.New(&runtime.Context{
		Logger:        &logger,
		TenantContext: &tenantctx.EnvironmentContext{Forwarders: []tenantctx.Forwarder{{ChainSelector: 1, Address: "0x"}}},
	})
	cmd.SetArgs([]string{"--output", "yaml"})
	require.Error(t, cmd.Execute())
}
