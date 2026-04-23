package simulate

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"

	"github.com/smartcontractkit/chainlink-common/pkg/beholder"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	httptypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	commonsettings "github.com/smartcontractkit/chainlink-common/pkg/settings"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
	pb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	simulator "github.com/smartcontractkit/chainlink/v2/core/services/workflows/cmd/cre/utils"
	v2 "github.com/smartcontractkit/chainlink/v2/core/services/workflows/v2"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WasmPath      string                       `validate:"omitempty,file,ascii,max=97" cli:"--wasm"`
	WorkflowPath  string                       `validate:"required,workflow_path_read"`
	ConfigPath    string                       `validate:"omitempty,file,ascii,max=97"`
	SecretsPath   string                       `validate:"omitempty,file,ascii,max=97"`
	EngineLogs    bool                         `validate:"omitempty" cli:"--engine-logs"`
	Broadcast     bool                         `validate:"-"`
	EVMClients    map[uint64]*ethclient.Client `validate:"omitempty"` // multichain clients keyed by selector (or chain ID for experimental)
	EthPrivateKey *ecdsa.PrivateKey            `validate:"omitempty"`
	WorkflowName  string                       `validate:"required"`
	// Non-interactive mode options
	NonInteractive bool   `validate:"-"`
	TriggerIndex   int    `validate:"-"`
	HTTPPayload    string `validate:"-"` // JSON string or @/path/to/file.json
	EVMTxHash      string `validate:"-"` // 0x-prefixed
	EVMEventIndex  int    `validate:"-"`
	// Experimental chains support (for chains not in official chain-selectors)
	ExperimentalForwarders map[uint64]common.Address `validate:"-"` // forwarders keyed by chain ID
	// Limits enforcement
	LimitsPath string `validate:"-"` // "default" or path to custom limits JSON
	// SkipTypeChecks passes --skip-type-checks to cre-compile for TypeScript workflows.
	SkipTypeChecks bool `validate:"-"`
	// InvocationDir is the working directory at the time the CLI was invoked, before
	// SetExecutionContext changes it to the workflow directory. Used to resolve file
	// paths entered interactively or via flags relative to where the user ran the command.
	InvocationDir string `validate:"-"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var simulateCmd = &cobra.Command{
		Use:     "simulate <workflow-folder-path>",
		Short:   "Simulates a workflow",
		Long:    `This command simulates a workflow.`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow simulate ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(runtimeContext.Viper, runtimeContext.Settings)
			if err != nil {
				return err
			}
			err = handler.ValidateInputs(inputs)
			if err != nil {
				return err
			}
			return handler.Execute(inputs)
		},
	}

	simulateCmd.Flags().BoolP("engine-logs", "g", false, "Enable non-fatal engine logging")
	simulateCmd.Flags().Bool("broadcast", false, "Broadcast transactions to the EVM (default: false)")
	simulateCmd.Flags().String("wasm", "", "Path or URL to a pre-built WASM binary (skips compilation)")
	simulateCmd.Flags().String("config", "", "Override the config file path from workflow.yaml")
	simulateCmd.Flags().Bool("no-config", false, "Simulate without a config file")
	simulateCmd.Flags().Bool("default-config", false, "Use the config path from workflow.yaml settings (default behavior)")
	simulateCmd.MarkFlagsMutuallyExclusive("config", "no-config", "default-config")
	// Non-interactive trigger selection flags
	simulateCmd.Flags().Int("trigger-index", -1, "Index of the trigger to run (0-based)")
	simulateCmd.Flags().String("http-payload", "", "HTTP trigger payload as JSON string or path to JSON file (with or without @ prefix)")
	simulateCmd.Flags().String("evm-tx-hash", "", "EVM trigger transaction hash (0x...)")
	simulateCmd.Flags().Int("evm-event-index", -1, "EVM trigger log index (0-based)")
	simulateCmd.Flags().String("limits", "default", "Production limits to enforce during simulation: 'default' for prod defaults, path to a limits JSON file (e.g. from 'cre workflow limits export'), or 'none' to disable")
	simulateCmd.Flags().Bool(cmdcommon.SkipTypeChecksCLIFlag, false, "Skip TypeScript project typecheck during compilation (passes "+cmdcommon.SkipTypeChecksFlag+" to cre-compile)")
	return simulateCmd
}

type handler struct {
	log            *zerolog.Logger
	runtimeContext *runtime.Context
	credentials    *credentials.Credentials
	validated      bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		runtimeContext: ctx,
		credentials:    ctx.Credentials,
		validated:      false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper, creSettings *settings.Settings) (Inputs, error) {
	// build clients for each supported chain from settings, skip if rpc is empty
	clients := make(map[uint64]*ethclient.Client)
	for _, chain := range SupportedEVM {
		chainName, err := settings.GetChainNameByChainSelector(chain.Selector)
		if err != nil {
			h.log.Error().Msgf("Invalid chain selector for supported EVM chains %d; skipping", chain.Selector)
			continue
		}
		rpcURL, err := settings.GetRpcUrlSettings(v, chainName)
		if err != nil || strings.TrimSpace(rpcURL) == "" {
			h.log.Debug().Msgf("RPC not provided for %s; skipping", chainName)
			continue
		}
		h.log.Debug().Msgf("Using RPC for %s: %s", chainName, redactURL(rpcURL))

		c, err := ethclient.Dial(rpcURL)
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to create eth client for %s: %v", chainName, err))
			continue
		}

		clients[chain.Selector] = c
	}

	// Experimental chains support (automatically loaded from config if present)
	experimentalForwarders := make(map[uint64]common.Address)

	expChains, err := settings.GetExperimentalChains(v)
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to load experimental chains config: %w", err)
	}

	for _, ec := range expChains {
		// Validate required fields
		if ec.ChainSelector == 0 {
			return Inputs{}, fmt.Errorf("experimental chain missing chain-selector")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return Inputs{}, fmt.Errorf("experimental chain %d missing rpc-url", ec.ChainSelector)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return Inputs{}, fmt.Errorf("experimental chain %d missing forwarder", ec.ChainSelector)
		}

		// Check if chain selector already exists (supported chain)
		if _, exists := clients[ec.ChainSelector]; exists {
			// Find the supported chain's forwarder
			var supportedForwarder string
			for _, supported := range SupportedEVM {
				if supported.Selector == ec.ChainSelector {
					supportedForwarder = supported.Forwarder
					break
				}
			}

			expFwd := common.HexToAddress(ec.Forwarder)
			if supportedForwarder != "" && common.HexToAddress(supportedForwarder) == expFwd {
				// Same forwarder, just debug log
				h.log.Debug().Uint64("chain-selector", ec.ChainSelector).Msg("Experimental chain matches supported chain config")
				continue
			}

			// Different forwarder - respect user's config, warn about override
			ui.Warning(fmt.Sprintf("Warning: experimental chain %d overrides supported chain forwarder (supported: %s, experimental: %s)\n", ec.ChainSelector, supportedForwarder, ec.Forwarder))

			// Use existing client but override the forwarder
			experimentalForwarders[ec.ChainSelector] = expFwd
			continue
		}

		// Dial the RPC
		h.log.Debug().Msgf("Using RPC for experimental chain %d: %s", ec.ChainSelector, redactURL(ec.RPCURL))
		c, err := ethclient.Dial(ec.RPCURL)
		if err != nil {
			return Inputs{}, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainSelector, err)
		}

		clients[ec.ChainSelector] = c
		experimentalForwarders[ec.ChainSelector] = common.HexToAddress(ec.Forwarder)
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-selector: %d)\n", ec.ChainSelector))

	}

	if len(clients) == 0 {
		return Inputs{}, fmt.Errorf("no RPC URLs found for supported or experimental chains")
	}

	pk, err := crypto.HexToECDSA(creSettings.User.EthPrivateKey)
	if err != nil {
		if v.GetBool("broadcast") {
			return Inputs{}, fmt.Errorf(
				"failed to parse private key, required to broadcast. Please check CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
		}
		pk, err = crypto.HexToECDSA("0000000000000000000000000000000000000000000000000000000000000001")
		if err != nil {
			return Inputs{}, fmt.Errorf("failed to parse default private key. Please set CRE_ETH_PRIVATE_KEY in your .env file or system environment: %w", err)
		}
		ui.Warning("Using default private key for chain write simulation. To use your own key, set CRE_ETH_PRIVATE_KEY in your .env file or system environment.")
	}

	return Inputs{
		WasmPath:               v.GetString("wasm"),
		WorkflowPath:           creSettings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		ConfigPath:             cmdcommon.ResolveConfigPath(v, creSettings.Workflow.WorkflowArtifactSettings.ConfigPath),
		SecretsPath:            creSettings.Workflow.WorkflowArtifactSettings.SecretsPath,
		EngineLogs:             v.GetBool("engine-logs"),
		Broadcast:              v.GetBool("broadcast"),
		EVMClients:             clients,
		EthPrivateKey:          pk,
		WorkflowName:           creSettings.Workflow.UserWorkflowSettings.WorkflowName,
		NonInteractive:         v.GetBool("non-interactive"),
		TriggerIndex:           v.GetInt("trigger-index"),
		HTTPPayload:            v.GetString("http-payload"),
		EVMTxHash:              v.GetString("evm-tx-hash"),
		EVMEventIndex:          v.GetInt("evm-event-index"),
		ExperimentalForwarders: experimentalForwarders,
		LimitsPath:             v.GetString("limits"),
		SkipTypeChecks:         v.GetBool(cmdcommon.SkipTypeChecksCLIFlag),
		InvocationDir:          h.runtimeContext.InvocationDir,
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	// URLs bypass the struct-level file/ascii/max validators.
	savedWasm := inputs.WasmPath
	savedConfig := inputs.ConfigPath
	if cmdcommon.IsURL(inputs.WasmPath) {
		inputs.WasmPath = ""
	}
	if cmdcommon.IsURL(inputs.ConfigPath) {
		inputs.ConfigPath = ""
	}

	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err = validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	inputs.WasmPath = savedWasm
	inputs.ConfigPath = savedConfig

	// forbid the default 0x...01 key when broadcasting
	if inputs.Broadcast && inputs.EthPrivateKey != nil {
		keyBytes, keyBytesErr := inputs.EthPrivateKey.Bytes()
		if keyBytesErr == nil && new(big.Int).SetBytes(keyBytes).Cmp(big.NewInt(1)) == 0 {
			return fmt.Errorf("you must configure a valid private key to perform on-chain writes. Please set your private key in the .env file before using the -–broadcast flag")
		}
	}

	rpcErr := ui.WithSpinner("Checking RPC connectivity...", func() error {
		return runRPCHealthCheck(inputs.EVMClients, inputs.ExperimentalForwarders)
	})
	if rpcErr != nil {
		// we don't block execution, just show the error to the user
		// because some RPCs in settings might not be used in workflow and some RPCs might have hiccups
		ui.Warning(fmt.Sprintf("Some RPCs in settings are not functioning properly, please check: %v", rpcErr))
	}

	h.validated = true
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	var wasmFileBinary []byte
	var err error

	if inputs.WasmPath != "" {
		if cmdcommon.IsURL(inputs.WasmPath) {
			ui.Dim("Fetching WASM binary from URL...")
			wasmFileBinary, err = cmdcommon.FetchURL(inputs.WasmPath)
			if err != nil {
				return fmt.Errorf("failed to fetch WASM from URL: %w", err)
			}
			ui.Success("Fetched WASM binary from URL")
		} else {
			ui.Dim("Reading pre-built WASM binary...")
			wasmFileBinary, err = os.ReadFile(inputs.WasmPath)
			if err != nil {
				return fmt.Errorf("failed to read WASM binary: %w", err)
			}
			ui.Success(fmt.Sprintf("Loaded WASM binary from %s", inputs.WasmPath))
		}
		wasmFileBinary, err = cmdcommon.EnsureRawWasm(wasmFileBinary)
		if err != nil {
			return fmt.Errorf("failed to decode WASM binary: %w", err)
		}
		if h.runtimeContext != nil {
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageWasm
		}
	} else {
		workflowDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("workflow directory: %w", err)
		}
		resolvedWorkflowPath, err := cmdcommon.ResolveWorkflowPath(workflowDir, inputs.WorkflowPath)
		if err != nil {
			return fmt.Errorf("workflow path: %w", err)
		}
		_, workflowMainFile, err := cmdcommon.WorkflowPathRootAndMain(resolvedWorkflowPath)
		if err != nil {
			return fmt.Errorf("workflow path: %w", err)
		}
		if h.runtimeContext != nil {
			h.runtimeContext.Workflow.Language = cmdcommon.GetWorkflowLanguage(workflowMainFile)
		}

		spinner := ui.NewSpinner()
		spinner.Start("Compiling workflow...")
		wasmFileBinary, err = cmdcommon.CompileWorkflowToWasm(resolvedWorkflowPath, cmdcommon.WorkflowCompileOptions{
			StripSymbols:   false,
			SkipTypeChecks: inputs.SkipTypeChecks,
		})
		spinner.Stop()
		if err != nil {
			ui.Error("Build failed:")
			return fmt.Errorf("failed to compile workflow: %w", err)
		}
		h.log.Debug().Msg("Workflow compiled")
		ui.Success("Workflow compiled")
	}

	// Resolve simulation limits
	simLimits, err := ResolveLimits(inputs.LimitsPath)
	if err != nil {
		return fmt.Errorf("failed to resolve simulation limits: %w", err)
	}

	// WASM binary size pre-flight check
	if simLimits != nil {
		binaryLimit := simLimits.WASMBinarySize()
		if binaryLimit > 0 && len(wasmFileBinary) > binaryLimit {
			return fmt.Errorf("WASM binary size %d bytes exceeds limit of %d bytes", len(wasmFileBinary), binaryLimit)
		}

		compressedLimit := simLimits.WASMCompressedBinarySize()
		if compressedLimit > 0 {
			compressed, err := cmdcommon.CompressBrotli(wasmFileBinary)
			if err != nil {
				return fmt.Errorf("failed to compress brotli: %w", err)
			}
			if len(compressed) > compressedLimit {
				return fmt.Errorf("WASM compressed binary size %d bytes exceeds limit of %d bytes", len(compressed), compressedLimit)
			}
		}

		ui.Success("Simulation limits enabled")
		ui.Dim(simLimits.LimitsSummary())
	}

	// Read the config file
	var config []byte
	if cmdcommon.IsURL(inputs.ConfigPath) {
		ui.Dim("Fetching config from URL...")
		config, err = cmdcommon.FetchURL(inputs.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to fetch config from URL: %w", err)
		}
		ui.Success("Fetched config from URL")
	} else if inputs.ConfigPath != "" {
		config, err = os.ReadFile(inputs.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	ui.Dim(fmt.Sprintf("Binary hash: %s", cmdcommon.HashBytes(wasmFileBinary)))
	ui.Dim(fmt.Sprintf("Config hash: %s", cmdcommon.HashBytes(config)))

	// Read the secrets file
	var secrets []byte
	if inputs.SecretsPath != "" {
		secrets, err = os.ReadFile(inputs.SecretsPath)
		if err != nil {
			return fmt.Errorf("failed to read secrets file: %w", err)
		}

		secrets, err = ReplaceSecretNamesWithEnvVars(secrets)
		if err != nil {
			return fmt.Errorf("failed to replace secret names with environment variables: %w", err)
		}
	}

	// Set up context for signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
	defer cancel()

	// if logger instance is set to DEBUG, that means verbosity flag is set by the user
	verbosity := h.log.GetLevel() == zerolog.DebugLevel

	err = run(ctx, wasmFileBinary, config, secrets, inputs, verbosity, simLimits)
	if err != nil {
		return err
	}

	h.showDeployAccessHint()

	return nil
}

func (h *handler) showDeployAccessHint() {
	if h.credentials == nil {
		return
	}

	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		return
	}

	if !deployAccess.HasAccess {
		ui.Line()
		message := ui.RenderSuccess("Simulation complete!") + " Ready to deploy your workflow?\n\n" +
			"Run " + ui.RenderCommand("cre account access") + " to request deployment access."
		ui.Box(message)
	}
}

// run instantiates the engine, starts it and blocks until the context is canceled.
func run(
	ctx context.Context,
	binary, config, secrets []byte,
	inputs Inputs,
	verbosity bool,
	simLimits *SimulationLimits,
) error {
	logCfg := logger.Config{Level: getLevel(verbosity, zapcore.InfoLevel)}
	simLogger := NewSimulationLogger(verbosity)

	engineLogCfg := logger.Config{Level: zapcore.FatalLevel}

	if inputs.EngineLogs {
		engineLogCfg.Level = logCfg.Level
	}

	engineLog, err := engineLogCfg.New()
	if err != nil {
		return fmt.Errorf("failed to create engine logger: %w", err)
	}

	// Channels to coordinate blocking
	initializedCh := make(chan struct{})
	executionFinishedCh := make(chan struct{})

	var triggerCaps *ManualTriggers
	simulatorInitialize := func(ctx context.Context, cfg simulator.RunnerConfig) (*capabilities.Registry, []services.Service) {
		lggr := logger.Sugared(cfg.Lggr)
		// Create the registry and fake capabilities with specific loggers
		registryLggr := lggr.Named("Registry")
		registry := capabilities.NewRegistry(registryLggr)
		registry.SetLocalRegistry(&capabilities.TestMetadataRegistry{})

		srvcs := []services.Service{}
		if cfg.EnableBilling {
			billingLggr := lggr.Named("Fake_Billing_Client")
			bs := simulator.NewBillingService(billingLggr)
			err := bs.Start(ctx)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to start billing service: %v", err))
				os.Exit(1)
			}

			srvcs = append(srvcs, bs)
		}

		if cfg.EnableBeholder {
			beholderLggr := lggr.Named("Beholder")
			err := setupCustomBeholder(beholderLggr, verbosity, simLogger)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to setup beholder: %v", err))
				os.Exit(1)
			}
		}

		// Build forwarder address map based on which chains actually have RPC clients configured
		forwarders := map[uint64]common.Address{}
		for _, c := range SupportedEVM {
			if _, ok := inputs.EVMClients[c.Selector]; ok && strings.TrimSpace(c.Forwarder) != "" {
				forwarders[c.Selector] = common.HexToAddress(c.Forwarder)
			}
		}

		// Merge experimental forwarders (keyed by chain ID)
		for chainID, fwdAddr := range inputs.ExperimentalForwarders {
			forwarders[chainID] = fwdAddr
		}

		manualTriggerCapConfig := ManualTriggerCapabilitiesConfig{
			Clients:    inputs.EVMClients,
			PrivateKey: inputs.EthPrivateKey,
			Forwarders: forwarders,
		}

		triggerLggr := lggr.Named("TriggerCapabilities")
		var err error
		triggerCaps, err = NewManualTriggerCapabilities(ctx, triggerLggr, registry, manualTriggerCapConfig, !inputs.Broadcast, simLimits)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create trigger capabilities: %v", err))
			os.Exit(1)
		}

		computeLggr := lggr.Named("ActionsCapabilities")
		computeCaps, err := NewFakeActionCapabilities(ctx, computeLggr, registry, inputs.SecretsPath, simLimits)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create compute capabilities: %v", err))
			os.Exit(1)
		}

		// Start trigger capabilities
		if err := triggerCaps.Start(ctx); err != nil {
			ui.Error(fmt.Sprintf("Failed to start trigger: %v", err))
			os.Exit(1)
		}

		// Start compute capabilities
		for _, cap := range computeCaps {
			if err = cap.Start(ctx); err != nil {
				ui.Error(fmt.Sprintf("Failed to start capability: %v", err))
				os.Exit(1)
			}
		}

		srvcs = append(srvcs, triggerCaps.ManualCronTrigger, triggerCaps.ManualHTTPTrigger)
		for _, evm := range triggerCaps.ManualEVMChains {
			srvcs = append(srvcs, evm)
		}
		srvcs = append(srvcs, computeCaps...)
		return registry, srvcs
	}

	// Create a holder for trigger info that will be populated in beforeStart
	triggerInfoAndBeforeStart := &TriggerInfoAndBeforeStart{}

	getTriggerCaps := func() *ManualTriggers { return triggerCaps }
	if inputs.NonInteractive {
		triggerInfoAndBeforeStart.BeforeStart = makeBeforeStartNonInteractive(triggerInfoAndBeforeStart, inputs, getTriggerCaps)
	} else {
		triggerInfoAndBeforeStart.BeforeStart = makeBeforeStartInteractive(triggerInfoAndBeforeStart, inputs, getTriggerCaps)
	}

	waitFn := func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service) {
		<-initializedCh

		// Manual trigger execution
		if triggerInfoAndBeforeStart.TriggerFunc == nil {
			simLogger.Error("Trigger function not initialized")
			os.Exit(1)
		}
		if triggerInfoAndBeforeStart.TriggerToRun == nil {
			simLogger.Error("Trigger to run not selected")
			os.Exit(1)
		}
		simLogger.Info("Running trigger", "trigger", triggerInfoAndBeforeStart.TriggerToRun.GetId())
		err := triggerInfoAndBeforeStart.TriggerFunc()
		if err != nil {
			simLogger.Error("Failed to run trigger", "trigger", triggerInfoAndBeforeStart.TriggerToRun.GetId(), "error", err)
			os.Exit(1)
		}

		select {
		case <-executionFinishedCh:
			simLogger.Info("Execution finished signal received")
		case <-ctx.Done():
			simLogger.Info("Received interrupt signal, stopping execution")
		case <-time.After(WorkflowExecutionTimeout):
			simLogger.Warn("Timeout waiting for execution to finish")
		}
	}
	simulatorCleanup := func(ctx context.Context, cfg simulator.RunnerConfig, registry *capabilities.Registry, services []services.Service) {
		for _, service := range services {
			if service.Name() == "WorkflowEngine.WorkflowEngineV2" {
				simLogger.Info("Skipping WorkflowEngineV2")
				continue
			}

			if err := service.Close(); err != nil {
				simLogger.Error("Failed to close service", "service", service.Name(), "error", err)
			}
		}

		err := cleanupBeholder()
		if err != nil {
			simLogger.Warn("Failed to cleanup beholder", "error", err)
		}
	}
	emptyHook := func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service) {}

	simulator.NewRunner(&simulator.RunnerHooks{
		Initialize:  simulatorInitialize,
		BeforeStart: triggerInfoAndBeforeStart.BeforeStart,
		Wait:        waitFn,
		AfterRun:    emptyHook,
		Cleanup:     simulatorCleanup,
		Finally:     emptyHook,
	}).Run(ctx, inputs.WorkflowName, binary, config, secrets, simulator.RunnerConfig{
		EnableBeholder: true,
		EnableBilling:  false,
		Lggr:           engineLog,
		LifecycleHooks: v2.LifecycleHooks{
			OnInitialized: func(err error) {
				if err != nil {
					simLogger.Error("Failed to initialize simulator", "error", err)
					os.Exit(1)
				}
				simLogger.Info("Simulator Initialized")
				ui.Line()
				close(initializedCh)
			},
			OnExecutionError: func(msg string) {
				ui.Error("Workflow execution failed:")
				ui.Print(msg)
				os.Exit(1)
			},
			OnResultReceived: func(result *pb.ExecutionResult) {
				if result == nil || result.Result == nil {
					// OnExecutionError will print the error message of the crash.
					return
				}

				ui.Line()
				switch r := result.Result.(type) {
				case *pb.ExecutionResult_Value:
					v, err := values.FromProto(r.Value)
					if err != nil {
						ui.Error("Could not decode result")
						break
					}

					uw, err := v.Unwrap()
					if err != nil {
						ui.Error(fmt.Sprintf("Could not unwrap result: %v", err))
						break
					}

					j, err := json.MarshalIndent(uw, "", "  ")
					if err != nil {
						ui.Error("Could not json marshal the result")
						break
					}

					ui.Success("Workflow Simulation Result:")
					ui.Print(string(j))
				case *pb.ExecutionResult_Error:
					ui.Error("Execution resulted in an error being returned: " + r.Error)
				}
				ui.Line()
				close(executionFinishedCh)
			},
		},
		WorkflowSettingsCfgFn: func(cfg *cresettings.Workflows) {
			// Apply simulation limits to engine-level settings when --limits is set
			if simLimits != nil {
				applyEngineLimits(cfg, simLimits)
			} else if inputs.LimitsPath == "none" {
				disableEngineLimits(cfg)
			}
			// Always allow all chains in simulation, overriding any chain restrictions from limits
			cfg.ChainAllowed = commonsettings.PerChainSelector(
				commonsettings.Bool(true),
				map[string]bool{},
			)
		},
	})

	return nil
}

type TriggerInfoAndBeforeStart struct {
	TriggerFunc  func() error
	TriggerToRun *pb.TriggerSubscription
	BeforeStart  func(ctx context.Context, cfg simulator.RunnerConfig, registry *capabilities.Registry, services []services.Service, triggerSub []*pb.TriggerSubscription)
}

// makeBeforeStartInteractive builds the interactive BeforeStart closure
func makeBeforeStartInteractive(holder *TriggerInfoAndBeforeStart, inputs Inputs, triggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
	return func(
		ctx context.Context,
		cfg simulator.RunnerConfig,
		registry *capabilities.Registry,
		services []services.Service,
		triggerSub []*pb.TriggerSubscription,
	) {
		if len(triggerSub) == 0 {
			ui.Error("No workflow triggers found, please check your workflow source code and config")
			os.Exit(1)
		}

		var triggerIndex int
		if len(triggerSub) > 1 {
			opts := make([]ui.SelectOption[int], len(triggerSub))
			for i, trigger := range triggerSub {
				opts[i] = ui.SelectOption[int]{
					Label: fmt.Sprintf("%s %s", trigger.GetId(), trigger.GetMethod()),
					Value: i,
				}
			}

			ui.Line()
			selected, err := ui.Select("Workflow simulation ready. Please select a trigger:", opts)
			if err != nil {
				ui.Error(fmt.Sprintf("Trigger selection failed: %v", err))
				os.Exit(1)
			}
			triggerIndex = selected

			holder.TriggerToRun = triggerSub[triggerIndex]
			ui.Line()
		} else {
			holder.TriggerToRun = triggerSub[0]
		}

		triggerRegistrationID := fmt.Sprintf("trigger_reg_1111111111111111111111111111111111111111111111111111111111111111_%d", triggerIndex)
		trigger := holder.TriggerToRun.Id
		triggerCaps := triggerCapsGetter()

		switch {
		case trigger == "cron-trigger@1.0.0":
			holder.TriggerFunc = func() error {
				skipWaitSignal := make(chan struct{}, 1)

				userPromptCtx, cancel := context.WithCancel(ctx)
				defer cancel()

				go func() {
					ui.Line()
					pressed := ui.WaitForEnter(userPromptCtx, "Cron scheduler started. Press Enter to skip waiting...")
					if pressed {
						skipWaitSignal <- struct{}{}
					}
				}()

				err := triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, skipWaitSignal)
				if err != nil {
					return err
				}

				return nil
			}
		case trigger == "http-trigger@1.0.0-alpha":
			payload, err := getHTTPTriggerPayload(inputs.InvocationDir)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to get HTTP trigger payload: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		case strings.HasPrefix(trigger, "evm") && strings.HasSuffix(trigger, "@1.0.0"):
			// Derive the chain selector directly from the selected trigger ID.
			sel, ok := parseChainSelectorFromTriggerID(holder.TriggerToRun.GetId())
			if !ok {
				ui.Error(fmt.Sprintf("Could not determine chain selector from trigger id %q", holder.TriggerToRun.GetId()))
				os.Exit(1)
			}

			client := inputs.EVMClients[sel]
			if client == nil {
				ui.Error(fmt.Sprintf("No RPC configured for chain selector %d", sel))
				os.Exit(1)
			}

			log, err := getEVMTriggerLog(ctx, client)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to get EVM trigger log: %v", err))
				os.Exit(1)
			}
			evmChain := triggerCaps.ManualEVMChains[sel]
			if evmChain == nil {
				ui.Error(fmt.Sprintf("No EVM chain initialized for selector %d", sel))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return evmChain.ManualTrigger(ctx, triggerRegistrationID, log)
			}
		default:
			ui.Error(fmt.Sprintf("Unsupported trigger type: %s", holder.TriggerToRun.Id))
			os.Exit(1)
		}
	}
}

// makeBeforeStartNonInteractive builds the non-interactive BeforeStart closure
func makeBeforeStartNonInteractive(holder *TriggerInfoAndBeforeStart, inputs Inputs, triggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
	return func(
		ctx context.Context,
		cfg simulator.RunnerConfig,
		registry *capabilities.Registry,
		services []services.Service,
		triggerSub []*pb.TriggerSubscription,
	) {
		if len(triggerSub) == 0 {
			ui.Error("No workflow triggers found, please check your workflow source code and config")
			os.Exit(1)
		}
		if inputs.TriggerIndex < 0 {
			ui.Error("--trigger-index is required when --non-interactive is enabled")
			os.Exit(1)
		}
		if inputs.TriggerIndex >= len(triggerSub) {
			ui.Error(fmt.Sprintf("Invalid --trigger-index %d; available range: 0-%d", inputs.TriggerIndex, len(triggerSub)-1))
			os.Exit(1)
		}

		holder.TriggerToRun = triggerSub[inputs.TriggerIndex]
		triggerRegistrationID := fmt.Sprintf("trigger_reg_1111111111111111111111111111111111111111111111111111111111111111_%d", inputs.TriggerIndex)
		trigger := holder.TriggerToRun.Id
		triggerCaps := triggerCapsGetter()

		switch {
		case trigger == "cron-trigger@1.0.0":
			holder.TriggerFunc = func() error {
				skipWaitSignal := make(chan struct{}, 1)
				err := triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, skipWaitSignal)
				if err != nil {
					return err
				}

				// With cron schedule on non-interactive mode
				skipWaitSignal <- struct{}{}

				return nil
			}
		case trigger == "http-trigger@1.0.0-alpha":
			if strings.TrimSpace(inputs.HTTPPayload) == "" {
				ui.Error("--http-payload is required for http-trigger@1.0.0-alpha in non-interactive mode")
				os.Exit(1)
			}
			payload, err := getHTTPTriggerPayloadFromInput(inputs.HTTPPayload, inputs.InvocationDir)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to parse HTTP trigger payload: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		case strings.HasPrefix(trigger, "evm") && strings.HasSuffix(trigger, "@1.0.0"):
			if strings.TrimSpace(inputs.EVMTxHash) == "" || inputs.EVMEventIndex < 0 {
				ui.Error("--evm-tx-hash and --evm-event-index are required for EVM triggers in non-interactive mode")
				os.Exit(1)
			}

			sel, ok := parseChainSelectorFromTriggerID(holder.TriggerToRun.GetId())
			if !ok {
				ui.Error(fmt.Sprintf("Could not determine chain selector from trigger id %q", holder.TriggerToRun.GetId()))
				os.Exit(1)
			}

			client := inputs.EVMClients[sel]
			if client == nil {
				ui.Error(fmt.Sprintf("No RPC configured for chain selector %d", sel))
				os.Exit(1)
			}

			log, err := getEVMTriggerLogFromValues(ctx, client, inputs.EVMTxHash, uint64(inputs.EVMEventIndex)) // #nosec G115 -- EVMEventIndex validated >= 0 above
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to build EVM trigger log: %v", err))
				os.Exit(1)
			}
			evmChain := triggerCaps.ManualEVMChains[sel]
			if evmChain == nil {
				ui.Error(fmt.Sprintf("No EVM chain initialized for selector %d", sel))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return evmChain.ManualTrigger(ctx, triggerRegistrationID, log)
			}
		default:
			ui.Error(fmt.Sprintf("Unsupported trigger type: %s", holder.TriggerToRun.Id))
			os.Exit(1)
		}
	}
}

// getLevel returns the default zapcore.Level unless verbosity flag is set by the user, then it sets it to DebugLevel
func getLevel(verbosity bool, defaultLevel zapcore.Level) zapcore.Level {
	if verbosity {
		return zapcore.DebugLevel
	}
	return defaultLevel
}

// setupCustomBeholder sets up beholder with our custom telemetry writer
func setupCustomBeholder(lggr logger.Logger, verbosity bool, simLogger *SimulationLogger) error {
	writer := &telemetryWriter{lggr: lggr, verbose: verbosity, simLogger: simLogger}

	client, err := beholder.NewWriterClient(writer)
	if err != nil {
		return err
	}

	beholder.SetClient(client)

	return nil
}

func cleanupBeholder() error {
	client := beholder.GetClient()
	if client != nil {
		return client.Close()
	}

	return nil
}

// getHTTPTriggerPayload prompts user for HTTP trigger data.
// invocationDir is the working directory at the time the CLI was invoked; relative
// paths entered by the user are resolved against it rather than the current working
// directory (which may have been changed to the workflow folder by SetExecutionContext).
func getHTTPTriggerPayload(invocationDir string) (*httptypedapi.Payload, error) {
	ui.Line()
	input, err := ui.Input("HTTP Trigger Configuration",
		ui.WithInputDescription("Enter a file path or JSON directly for the HTTP trigger"),
		ui.WithPlaceholder(`{"key": "value"} or ./payload.json`),
	)
	if err != nil {
		return nil, fmt.Errorf("HTTP trigger input cancelled: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input provided")
	}

	var jsonData map[string]interface{}

	// Resolve the path against the invocation directory so that relative paths
	// like ./production.json work from where the user ran the command, even though
	// the process cwd has been changed to the workflow subdirectory.
	resolvedPath := resolvePathFromInvocation(input, invocationDir)

	if _, err := os.Stat(resolvedPath); err == nil {
		data, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", resolvedPath, err)
		}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON from file %s: %w", resolvedPath, err)
		}
		ui.Success(fmt.Sprintf("Loaded JSON from file: %s", resolvedPath))
	} else {
		// Treat as direct JSON input
		if err := json.Unmarshal([]byte(input), &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		ui.Success("Parsed JSON input successfully")
	}

	jsonDataBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	payload := &httptypedapi.Payload{
		Input: jsonDataBytes,
		// Key is optional for simulation
	}

	ui.Success(fmt.Sprintf("Created HTTP trigger payload with %d fields", len(jsonData)))
	return payload, nil
}

// resolvePathFromInvocation converts a (potentially relative) path to an absolute
// path anchored at invocationDir. Absolute paths and paths that are already
// reachable from the current working directory are returned unchanged.
func resolvePathFromInvocation(path, invocationDir string) string {
	if filepath.IsAbs(path) || invocationDir == "" {
		return path
	}
	return filepath.Join(invocationDir, path)
}

// getEVMTriggerLog prompts user for EVM trigger data and fetches the log
func getEVMTriggerLog(ctx context.Context, ethClient *ethclient.Client) (*evm.Log, error) {
	var txHashInput string
	var eventIndexInput string

	ui.Line()
	if err := ui.InputForm([]ui.InputField{
		{
			Title:       "EVM Trigger Configuration",
			Description: "Transaction hash for the EVM log event",
			Placeholder: "0x...",
			Value:       &txHashInput,
			Validate: func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("transaction hash cannot be empty")
				}
				if !strings.HasPrefix(s, "0x") {
					return fmt.Errorf("transaction hash must start with 0x")
				}
				if len(s) != 66 {
					return fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(s))
				}
				return nil
			},
		},
		{
			Title:       "Event Index",
			Description: "Log event index (0-based)",
			Placeholder: "0",
			Suggestions: []string{"0"},
			Value:       &eventIndexInput,
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("event index cannot be empty")
				}
				if _, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32); err != nil {
					return fmt.Errorf("invalid event index: must be a number")
				}
				return nil
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("EVM trigger input cancelled: %w", err)
	}

	txHashInput = strings.TrimSpace(txHashInput)
	txHash := common.HexToHash(txHashInput)

	eventIndexInput = strings.TrimSpace(eventIndexInput)
	eventIndex, err := strconv.ParseUint(eventIndexInput, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid event index: %w", err)
	}

	// Fetch the transaction receipt
	receiptSpinner := ui.NewSpinner()
	receiptSpinner.Start(fmt.Sprintf("Fetching transaction receipt for %s...", txHash.Hex()))
	txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
	receiptSpinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}

	// Check if event index is valid
	if eventIndex >= uint64(len(txReceipt.Logs)) {
		return nil, fmt.Errorf("event index %d out of range, transaction has %d log events", eventIndex, len(txReceipt.Logs))
	}

	log := txReceipt.Logs[eventIndex]
	ui.Success(fmt.Sprintf("Found log event at index %d: contract=%s, topics=%d", eventIndex, log.Address.Hex(), len(log.Topics)))

	// Check for potential uint32 overflow (prevents noisy linter warnings)
	var txIndex, logIndex uint32
	if log.TxIndex > math.MaxUint32 {
		return nil, fmt.Errorf("transaction index %d exceeds uint32 maximum value", log.TxIndex)
	}
	txIndex = uint32(log.TxIndex)

	if log.Index > math.MaxUint32 {
		return nil, fmt.Errorf("log index %d exceeds uint32 maximum value", log.Index)
	}
	logIndex = uint32(log.Index)

	// Convert to protobuf format
	pbLog := &evm.Log{
		Address:     log.Address.Bytes(),
		Data:        log.Data,
		BlockHash:   log.BlockHash.Bytes(),
		TxHash:      log.TxHash.Bytes(),
		TxIndex:     txIndex,
		Index:       logIndex,
		Removed:     log.Removed,
		BlockNumber: valuespb.NewBigIntFromInt(new(big.Int).SetUint64(log.BlockNumber)),
	}

	// Convert topics
	for _, topic := range log.Topics {
		pbLog.Topics = append(pbLog.Topics, topic.Bytes())
	}

	// Set event signature (first topic is usually the event signature)
	if len(log.Topics) > 0 {
		pbLog.EventSig = log.Topics[0].Bytes()
	}

	ui.Success(fmt.Sprintf("Created EVM trigger log for transaction %s, event %d", txHash.Hex(), eventIndex))
	return pbLog, nil
}

// getHTTPTriggerPayloadFromInput builds an HTTP trigger payload from a JSON string or a file path
// (optionally prefixed with '@'). invocationDir is used to resolve relative paths against the
// directory where the user invoked the CLI rather than the current working directory.
func getHTTPTriggerPayloadFromInput(input, invocationDir string) (*httptypedapi.Payload, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("empty http payload input")
	}

	var raw []byte
	if strings.HasPrefix(trimmed, "@") {
		path := resolvePathFromInvocation(strings.TrimPrefix(trimmed, "@"), invocationDir)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		raw = data
	} else {
		resolvedPath := resolvePathFromInvocation(trimmed, invocationDir)
		if _, err := os.Stat(resolvedPath); err == nil {
			data, err := os.ReadFile(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", resolvedPath, err)
			}
			raw = data
		} else {
			raw = []byte(trimmed)
		}
	}

	return &httptypedapi.Payload{Input: raw}, nil
}

// getEVMTriggerLogFromValues fetches a log given tx hash and event index
func getEVMTriggerLogFromValues(ctx context.Context, ethClient *ethclient.Client, txHashStr string, eventIndex uint64) (*evm.Log, error) {
	txHashStr = strings.TrimSpace(txHashStr)
	if txHashStr == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}
	if !strings.HasPrefix(txHashStr, "0x") {
		return nil, fmt.Errorf("transaction hash must start with 0x")
	}
	if len(txHashStr) != 66 { // 0x + 64 hex chars
		return nil, fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(txHashStr))
	}

	txHash := common.HexToHash(txHashStr)
	receiptSpinner := ui.NewSpinner()
	receiptSpinner.Start(fmt.Sprintf("Fetching transaction receipt for %s...", txHash.Hex()))
	txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
	receiptSpinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}
	if eventIndex >= uint64(len(txReceipt.Logs)) {
		return nil, fmt.Errorf("event index %d out of range, transaction has %d log events", eventIndex, len(txReceipt.Logs))
	}

	log := txReceipt.Logs[eventIndex]

	// Check for potential uint32 overflow
	var txIndex, logIndex uint32
	if log.TxIndex > math.MaxUint32 {
		return nil, fmt.Errorf("transaction index %d exceeds uint32 maximum value", log.TxIndex)
	}
	txIndex = uint32(log.TxIndex)
	if log.Index > math.MaxUint32 {
		return nil, fmt.Errorf("log index %d exceeds uint32 maximum value", log.Index)
	}
	logIndex = uint32(log.Index)

	pbLog := &evm.Log{
		Address:     log.Address.Bytes(),
		Data:        log.Data,
		BlockHash:   log.BlockHash.Bytes(),
		TxHash:      log.TxHash.Bytes(),
		TxIndex:     txIndex,
		Index:       logIndex,
		Removed:     log.Removed,
		BlockNumber: valuespb.NewBigIntFromInt(new(big.Int).SetUint64(log.BlockNumber)),
	}
	for _, topic := range log.Topics {
		pbLog.Topics = append(pbLog.Topics, topic.Bytes())
	}
	if len(log.Topics) > 0 {
		pbLog.EventSig = log.Topics[0].Bytes()
	}
	return pbLog, nil
}
