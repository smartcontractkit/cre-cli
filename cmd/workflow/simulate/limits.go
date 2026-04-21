package simulate

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/settings"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
)

//go:embed limits.json
var defaultLimitsJSON []byte

// SimulationLimits holds the workflow-level limits applied during simulation.
type SimulationLimits struct {
	Workflows cresettings.Workflows
}

// DefaultLimits returns simulation limits populated from the embedded defaults.
func DefaultLimits() (*SimulationLimits, error) {
	return parseLimitsJSON(defaultLimitsJSON)
}

// LoadLimits reads a limits JSON file from disk and returns parsed SimulationLimits.
func LoadLimits(path string) (*SimulationLimits, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read limits file %s: %w", path, err)
	}
	return parseLimitsJSON(data)
}

func parseLimitsJSON(data []byte) (*SimulationLimits, error) {
	// Start from the pre-built Default which has all Parse functions configured.
	// Setting[T].Parse is a function closure (json:"-") that cannot be serialized,
	// so we must unmarshal into a copy that already has Parse funcs set.
	w := cresettings.Default.PerWorkflow
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("failed to parse limits JSON: %w", err)
	}
	return &SimulationLimits{Workflows: w}, nil
}

// applyEngineLimits copies limit values from the SimulationLimits into the
// engine's workflow settings config. ChainAllowed is intentionally left as
// allow-all for simulation.
func applyEngineLimits(cfg *cresettings.Workflows, limits *SimulationLimits) {
	src := &limits.Workflows

	// Execution limits
	cfg.ExecutionTimeout = src.ExecutionTimeout
	cfg.ExecutionResponseLimit = src.ExecutionResponseLimit
	cfg.ExecutionConcurrencyLimit = src.ExecutionConcurrencyLimit

	// Capability limits
	cfg.CapabilityConcurrencyLimit = src.CapabilityConcurrencyLimit
	cfg.CapabilityCallTimeout = src.CapabilityCallTimeout
	cfg.SecretsConcurrencyLimit = src.SecretsConcurrencyLimit

	// Trigger limits
	cfg.TriggerRegistrationsTimeout = src.TriggerRegistrationsTimeout
	cfg.TriggerEventQueueLimit = src.TriggerEventQueueLimit
	cfg.TriggerEventQueueTimeout = src.TriggerEventQueueTimeout
	cfg.TriggerSubscriptionTimeout = src.TriggerSubscriptionTimeout
	cfg.TriggerSubscriptionLimit = src.TriggerSubscriptionLimit

	// WASM limits
	cfg.WASMMemoryLimit = src.WASMMemoryLimit
	cfg.WASMBinarySizeLimit = src.WASMBinarySizeLimit
	cfg.WASMCompressedBinarySizeLimit = src.WASMCompressedBinarySizeLimit
	cfg.WASMConfigSizeLimit = src.WASMConfigSizeLimit
	cfg.WASMSecretsSizeLimit = src.WASMSecretsSizeLimit

	// Log limits
	cfg.LogLineLimit = src.LogLineLimit
	cfg.LogEventLimit = src.LogEventLimit

	// Call count limits
	cfg.ChainRead = src.ChainRead
	cfg.ChainWrite.TargetsLimit = src.ChainWrite.TargetsLimit
	cfg.Consensus.CallLimit = src.Consensus.CallLimit
	cfg.HTTPAction.CallLimit = src.HTTPAction.CallLimit
	cfg.ConfidentialHTTP.CallLimit = src.ConfidentialHTTP.CallLimit
	cfg.Secrets = src.Secrets

	// Trigger-specific limits
	cfg.CRONTrigger = src.CRONTrigger
	cfg.HTTPTrigger = src.HTTPTrigger
	cfg.LogTrigger = src.LogTrigger

	// NOTE: ChainAllowed is NOT overridden — simulation keeps allow-all
}

// disableEngineLimits sets all engine limit fields to very high values,
// effectively disabling limit enforcement when --limits none is used.
func disableEngineLimits(cfg *cresettings.Workflows) {
	maxInt := settings.Setting[int]{DefaultValue: math.MaxInt32}
	maxSize := settings.Setting[config.Size]{DefaultValue: math.MaxInt32}
	maxDuration := settings.Setting[time.Duration]{DefaultValue: 24 * time.Hour}

	// Execution limits
	cfg.ExecutionTimeout = maxDuration
	cfg.ExecutionResponseLimit = maxSize
	cfg.ExecutionConcurrencyLimit = maxInt

	// Capability limits
	cfg.CapabilityConcurrencyLimit = maxInt
	cfg.CapabilityCallTimeout = maxDuration
	cfg.SecretsConcurrencyLimit = maxInt

	// Trigger limits
	cfg.TriggerRegistrationsTimeout = maxDuration
	cfg.TriggerEventQueueLimit = maxInt
	cfg.TriggerEventQueueTimeout = maxDuration
	cfg.TriggerSubscriptionTimeout = maxDuration
	cfg.TriggerSubscriptionLimit = maxInt

	// WASM limits
	cfg.WASMMemoryLimit = maxSize
	cfg.WASMBinarySizeLimit = maxSize
	cfg.WASMCompressedBinarySizeLimit = maxSize
	cfg.WASMConfigSizeLimit = maxSize
	cfg.WASMSecretsSizeLimit = maxSize

	// Log limits
	cfg.LogLineLimit = maxSize
	cfg.LogEventLimit = maxInt

	// HTTPAction limits
	cfg.HTTPAction.CallLimit = maxInt
	cfg.HTTPAction.CacheAgeLimit = maxDuration
	cfg.HTTPAction.ConnectionTimeout = maxDuration
	cfg.HTTPAction.RequestSizeLimit = maxSize
	cfg.HTTPAction.ResponseSizeLimit = maxSize

	// ConfidentialHTTP limits
	cfg.ConfidentialHTTP.CallLimit = maxInt
	cfg.ConfidentialHTTP.ConnectionTimeout = maxDuration
	cfg.ConfidentialHTTP.RequestSizeLimit = maxSize
	cfg.ConfidentialHTTP.ResponseSizeLimit = maxSize

	// Consensus limits
	cfg.Consensus.CallLimit = maxInt
	cfg.Consensus.ObservationSizeLimit = maxSize

	// ChainWrite limits
	cfg.ChainWrite.TargetsLimit = maxInt
	cfg.ChainWrite.ReportSizeLimit = maxSize

	// ChainRead limits
	cfg.ChainRead.CallLimit = maxInt
	cfg.ChainRead.PayloadSizeLimit = maxSize

	// Secrets limits
	cfg.Secrets.CallLimit = maxInt
}

// HTTPRequestSizeLimit returns the HTTP action request size limit in bytes.
func (l *SimulationLimits) HTTPRequestSizeLimit() int {
	return int(l.Workflows.HTTPAction.RequestSizeLimit.DefaultValue)
}

// HTTPResponseSizeLimit returns the HTTP action response size limit in bytes.
func (l *SimulationLimits) HTTPResponseSizeLimit() int {
	return int(l.Workflows.HTTPAction.ResponseSizeLimit.DefaultValue)
}

// ConfHTTPRequestSizeLimit returns the confidential HTTP request size limit in bytes.
func (l *SimulationLimits) ConfHTTPRequestSizeLimit() int {
	return int(l.Workflows.ConfidentialHTTP.RequestSizeLimit.DefaultValue)
}

// ConfHTTPResponseSizeLimit returns the confidential HTTP response size limit in bytes.
func (l *SimulationLimits) ConfHTTPResponseSizeLimit() int {
	return int(l.Workflows.ConfidentialHTTP.ResponseSizeLimit.DefaultValue)
}

// ConsensusObservationSizeLimit returns the consensus observation size limit in bytes.
func (l *SimulationLimits) ConsensusObservationSizeLimit() int {
	return int(l.Workflows.Consensus.ObservationSizeLimit.DefaultValue)
}

// ChainWriteReportSizeLimit returns the chain write report size limit in bytes.
func (l *SimulationLimits) ChainWriteReportSizeLimit() int {
	return int(l.Workflows.ChainWrite.ReportSizeLimit.DefaultValue)
}

// ChainWriteGasLimit returns the default EVM gas limit.
func (l *SimulationLimits) ChainWriteGasLimit() uint64 {
	return l.Workflows.ChainWrite.EVM.GasLimit.Default.DefaultValue
}

// ChainWriteAptosMaxGasAmount returns the default Aptos max_gas_amount per WriteReport.
func (l *SimulationLimits) ChainWriteAptosMaxGasAmount() uint64 {
	return l.Workflows.ChainWrite.Aptos.GasLimit.Default.DefaultValue
}

// WASMBinarySize returns the WASM binary size limit in bytes.
func (l *SimulationLimits) WASMBinarySize() int {
	return int(l.Workflows.WASMBinarySizeLimit.DefaultValue)
}

// WASMCompressedBinarySize returns the WASM compressed binary size limit in bytes.
func (l *SimulationLimits) WASMCompressedBinarySize() int {
	return int(l.Workflows.WASMCompressedBinarySizeLimit.DefaultValue)
}

// LimitsSummary returns a human-readable summary of key limits.
func (l *SimulationLimits) LimitsSummary() string {
	w := &l.Workflows
	return fmt.Sprintf(
		"HTTP: req=%s resp=%s timeout=%s | ConfHTTP: req=%s resp=%s timeout=%s | Consensus obs=%s | ChainWrite report=%s gas=%d | WASM binary=%s compressed=%s",
		w.HTTPAction.RequestSizeLimit.DefaultValue,
		w.HTTPAction.ResponseSizeLimit.DefaultValue,
		w.HTTPAction.ConnectionTimeout.DefaultValue,
		w.ConfidentialHTTP.RequestSizeLimit.DefaultValue,
		w.ConfidentialHTTP.ResponseSizeLimit.DefaultValue,
		w.ConfidentialHTTP.ConnectionTimeout.DefaultValue,
		w.Consensus.ObservationSizeLimit.DefaultValue,
		w.ChainWrite.ReportSizeLimit.DefaultValue,
		w.ChainWrite.EVM.GasLimit.Default.DefaultValue,
		w.WASMBinarySizeLimit.DefaultValue,
		w.WASMCompressedBinarySizeLimit.DefaultValue,
	)
}

// ExportDefaultLimitsJSON returns the embedded default limits JSON.
func ExportDefaultLimitsJSON() []byte {
	return defaultLimitsJSON
}

// ResolveLimits resolves a --limits flag value to SimulationLimits.
// Returns nil if limitsFlag is "none" (no limits enforcement).
func ResolveLimits(limitsFlag string) (*SimulationLimits, error) {
	if limitsFlag == "none" {
		return nil, nil
	}

	if strings.TrimSpace(limitsFlag) == "" || limitsFlag == "default" {
		return DefaultLimits()
	}

	return LoadLimits(limitsFlag)
}
