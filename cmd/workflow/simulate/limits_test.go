package simulate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
)

func writeLimitsFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "limits.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestDefaultLimitsAndExportDefaultLimitsJSON(t *testing.T) {
	t.Parallel()

	limits, err := DefaultLimits()
	require.NoError(t, err)

	assert.Equal(t, 10_000, limits.HTTPRequestSizeLimit())
	assert.Equal(t, 100_000, limits.HTTPResponseSizeLimit())
	assert.Equal(t, 10_000, limits.ConfHTTPRequestSizeLimit())
	assert.Equal(t, 100_000, limits.ConfHTTPResponseSizeLimit())
	assert.Equal(t, 100_000, limits.ConsensusObservationSizeLimit())
	assert.Equal(t, 5_000, limits.ChainWriteReportSizeLimit())
	assert.Equal(t, 100_000_000, limits.WASMBinarySize())
	assert.Equal(t, 20_000_000, limits.WASMCompressedBinarySize())
	assert.JSONEq(t, string(defaultLimitsJSON), string(ExportDefaultLimitsJSON()))
}

func TestLoadLimitsParsesCustomFileAndPreservesDefaultsForUnsetFields(t *testing.T) {
	t.Parallel()

	path := writeLimitsFile(t, `{
		"HTTPAction": {
			"RequestSizeLimit": "7kb",
			"ConnectionTimeout": "2s"
		},
		"ChainWrite": {
			"ReportSizeLimit": "9kb"
		},
		"CRONTrigger": {
			"FastestScheduleInterval": "45s"
		}
	}`)

	limits, err := LoadLimits(path)
	require.NoError(t, err)

	assert.Equal(t, 7_000, limits.HTTPRequestSizeLimit())
	assert.Equal(t, 100_000, limits.HTTPResponseSizeLimit(), "unset values should keep embedded defaults")
	assert.Equal(t, 9_000, limits.ChainWriteReportSizeLimit())
	assert.Equal(t, 45*time.Second, limits.Workflows.CRONTrigger.FastestScheduleInterval.DefaultValue)
	assert.Equal(t, 2*time.Second, limits.Workflows.HTTPAction.ConnectionTimeout.DefaultValue)
}

func TestLoadLimitsReturnsHelpfulErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadLimits(filepath.Join(t.TempDir(), "missing.json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read limits file")
	})

	t.Run("invalid json", func(t *testing.T) {
		path := writeLimitsFile(t, `{invalid json`)
		_, err := LoadLimits(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse limits JSON")
	})
}

func TestResolveLimitsHandlesAllSupportedModes(t *testing.T) {
	t.Parallel()

	flag := "none"
	limits, err := ResolveLimits(flag)
	require.NoError(t, err)
	assert.Nil(t, limits)

	defaultLimits, err := ResolveLimits("default")
	require.NoError(t, err)
	baseline, err := DefaultLimits()
	require.NoError(t, err)
	assert.Equal(t, baseline.HTTPRequestSizeLimit(), defaultLimits.HTTPRequestSizeLimit())
	assert.Equal(t, baseline.ChainWriteReportSizeLimit(), defaultLimits.ChainWriteReportSizeLimit())

	path := writeLimitsFile(t, `{"Consensus":{"ObservationSizeLimit":"2kb"}}`)
	customLimits, err := ResolveLimits(path)
	require.NoError(t, err)
	assert.Equal(t, 2_000, customLimits.ConsensusObservationSizeLimit())
}

func TestApplyEngineLimitsCopiesSupportedFieldsAndPreservesChainAllowed(t *testing.T) {
	t.Parallel()

	cfg := cresettings.Default.PerWorkflow
	cfg.ChainAllowed.Default.DefaultValue = true

	limits := newTestLimits(t)
	limits.Workflows.ExecutionTimeout.DefaultValue = 11 * time.Second
	limits.Workflows.ExecutionResponseLimit.DefaultValue = 2048
	limits.Workflows.ExecutionConcurrencyLimit.DefaultValue = 9
	limits.Workflows.CapabilityConcurrencyLimit.DefaultValue = 7
	limits.Workflows.CapabilityCallTimeout.DefaultValue = 12 * time.Second
	limits.Workflows.SecretsConcurrencyLimit.DefaultValue = 6
	limits.Workflows.TriggerRegistrationsTimeout.DefaultValue = 13 * time.Second
	limits.Workflows.TriggerEventQueueLimit.DefaultValue = 14
	limits.Workflows.TriggerEventQueueTimeout.DefaultValue = 15 * time.Second
	limits.Workflows.TriggerSubscriptionTimeout.DefaultValue = 16 * time.Second
	limits.Workflows.TriggerSubscriptionLimit.DefaultValue = 17
	limits.Workflows.WASMMemoryLimit.DefaultValue = 4096
	limits.Workflows.WASMBinarySizeLimit.DefaultValue = 8192
	limits.Workflows.WASMCompressedBinarySizeLimit.DefaultValue = 1024
	limits.Workflows.WASMConfigSizeLimit.DefaultValue = 512
	limits.Workflows.WASMSecretsSizeLimit.DefaultValue = 256
	limits.Workflows.LogLineLimit.DefaultValue = 128
	limits.Workflows.LogEventLimit.DefaultValue = 25
	limits.Workflows.ChainRead.CallLimit.DefaultValue = 3
	limits.Workflows.ChainWrite.TargetsLimit.DefaultValue = 4
	limits.Workflows.Consensus.CallLimit.DefaultValue = 5
	limits.Workflows.HTTPAction.CallLimit.DefaultValue = 6
	limits.Workflows.ConfidentialHTTP.CallLimit.DefaultValue = 7
	limits.Workflows.Secrets.CallLimit.DefaultValue = 8
	limits.Workflows.CRONTrigger.FastestScheduleInterval.DefaultValue = 30 * time.Second

	applyEngineLimits(&cfg, limits)

	assert.Equal(t, 11*time.Second, cfg.ExecutionTimeout.DefaultValue)
	assert.Equal(t, 2048, int(cfg.ExecutionResponseLimit.DefaultValue))
	assert.Equal(t, 9, cfg.ExecutionConcurrencyLimit.DefaultValue)
	assert.Equal(t, 7, cfg.CapabilityConcurrencyLimit.DefaultValue)
	assert.Equal(t, 12*time.Second, cfg.CapabilityCallTimeout.DefaultValue)
	assert.Equal(t, 6, cfg.SecretsConcurrencyLimit.DefaultValue)
	assert.Equal(t, 13*time.Second, cfg.TriggerRegistrationsTimeout.DefaultValue)
	assert.Equal(t, 14, cfg.TriggerEventQueueLimit.DefaultValue)
	assert.Equal(t, 15*time.Second, cfg.TriggerEventQueueTimeout.DefaultValue)
	assert.Equal(t, 16*time.Second, cfg.TriggerSubscriptionTimeout.DefaultValue)
	assert.Equal(t, 17, cfg.TriggerSubscriptionLimit.DefaultValue)
	assert.Equal(t, 4096, int(cfg.WASMMemoryLimit.DefaultValue))
	assert.Equal(t, 8192, int(cfg.WASMBinarySizeLimit.DefaultValue))
	assert.Equal(t, 1024, int(cfg.WASMCompressedBinarySizeLimit.DefaultValue))
	assert.Equal(t, 512, int(cfg.WASMConfigSizeLimit.DefaultValue))
	assert.Equal(t, 256, int(cfg.WASMSecretsSizeLimit.DefaultValue))
	assert.Equal(t, 128, int(cfg.LogLineLimit.DefaultValue))
	assert.Equal(t, 25, cfg.LogEventLimit.DefaultValue)
	assert.Equal(t, 3, cfg.ChainRead.CallLimit.DefaultValue)
	assert.Equal(t, 4, cfg.ChainWrite.TargetsLimit.DefaultValue)
	assert.Equal(t, 5, cfg.Consensus.CallLimit.DefaultValue)
	assert.Equal(t, 6, cfg.HTTPAction.CallLimit.DefaultValue)
	assert.Equal(t, 7, cfg.ConfidentialHTTP.CallLimit.DefaultValue)
	assert.Equal(t, 8, cfg.Secrets.CallLimit.DefaultValue)
	assert.Equal(t, 30*time.Second, cfg.CRONTrigger.FastestScheduleInterval.DefaultValue)
	assert.True(t, cfg.ChainAllowed.Default.DefaultValue, "simulation should preserve allow-all ChainAllowed settings")
}

func TestSimulationLimitsSummaryIncludesKeyLimitValues(t *testing.T) {
	t.Parallel()

	summary := newTestLimits(t).LimitsSummary()
	assert.Contains(t, summary, "HTTP: req=10kb resp=100kb timeout=10s")
	assert.Contains(t, summary, "ConfHTTP: req=10kb resp=100kb timeout=10s")
	assert.Contains(t, summary, "Consensus obs=100kb")
	assert.Contains(t, summary, "ChainWrite report=5kb evm_gas=5000000")
	assert.Contains(t, summary, "aptos_gas=")
	assert.Contains(t, summary, "WASM binary=100mb compressed=20mb")
}
