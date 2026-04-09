package simulate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap/zapcore"

	"github.com/smartcontractkit/chainlink-common/pkg/beholder"
	httptypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	commonsettings "github.com/smartcontractkit/chainlink-common/pkg/settings"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
	pb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	simulator "github.com/smartcontractkit/chainlink/v2/core/services/workflows/cmd/cre/utils"
	v2 "github.com/smartcontractkit/chainlink/v2/core/services/workflows/v2"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chainfamily"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WasmPath     string `validate:"omitempty,file,ascii,max=97" cli:"--wasm"`
	WorkflowPath string `validate:"required,workflow_path_read"`
	ConfigPath   string `validate:"omitempty,file,ascii,max=97"`
	SecretsPath  string `validate:"omitempty,file,ascii,max=97"`
	EngineLogs   bool   `validate:"omitempty" cli:"--engine-logs"`
	WorkflowName string `validate:"required"`
	// Non-interactive mode options
	NonInteractive bool   `validate:"-"`
	TriggerIndex   int    `validate:"-"`
	HTTPPayload    string `validate:"-"` // JSON string or @/path/to/file.json
	// Chain runtimes set up by adapters during ResolveInputs.
	Runtimes          map[string]chainfamily.ChainRuntime `validate:"-"`
	SelectorToRuntime map[uint64]chainfamily.ChainRuntime `validate:"-"`
	FlagValues        func(name string) string            `validate:"-"`
	// Limits enforcement
	LimitsPath string            `validate:"-"` // "default" or path to custom limits JSON
	SimLimits  *SimulationLimits `validate:"-"` // resolved limits (nil = no limits)
	// SkipTypeChecks passes --skip-type-checks to cre-compile for TypeScript workflows.
	SkipTypeChecks bool `validate:"-"`
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

			inputs, err := handler.ResolveInputs(cmd, runtimeContext.Viper, runtimeContext.Settings)
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
	simulateCmd.Flags().String("wasm", "", "Path or URL to a pre-built WASM binary (skips compilation)")
	simulateCmd.Flags().String("config", "", "Override the config file path from workflow.yaml")
	simulateCmd.Flags().Bool("no-config", false, "Simulate without a config file")
	simulateCmd.Flags().Bool("default-config", false, "Use the config path from workflow.yaml settings (default behavior)")
	simulateCmd.MarkFlagsMutuallyExclusive("config", "no-config", "default-config")
	// Non-interactive flags
	simulateCmd.Flags().Bool(settings.Flags.NonInteractive.Name, false, "Run without prompts; requires --trigger-index and inputs for the selected trigger type")
	simulateCmd.Flags().Int("trigger-index", -1, "Index of the trigger to run (0-based)")
	simulateCmd.Flags().String("http-payload", "", "HTTP trigger payload as JSON string or path to JSON file (with or without @ prefix)")
	simulateCmd.Flags().String("limits", "default", "Production limits to enforce during simulation: 'default' for prod defaults, path to a limits JSON file (e.g. from 'cre workflow limits export'), or 'none' to disable")
	simulateCmd.Flags().Bool(cmdcommon.SkipTypeChecksCLIFlag, false, "Skip TypeScript project typecheck during compilation (passes "+cmdcommon.SkipTypeChecksFlag+" to cre-compile)")

	// Let each registered chain family adapter add its own flags.
	for _, adapter := range chainfamily.All() {
		adapter.AddFlags(simulateCmd)
	}

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

func (h *handler) ResolveInputs(cmd *cobra.Command, v *viper.Viper, creSettings *settings.Settings) (Inputs, error) {
	// Build the FlagValues function that adapters use to read flag values and settings.
	flagValues := func(name string) string {
		// First try cobra flags
		if f := cmd.Flags().Lookup(name); f != nil {
			return f.Value.String()
		}
		// Fall back to viper (env vars, settings)
		return v.GetString(name)
	}

	// Resolve simulation limits early so they can be passed to adapter Setup.
	simLimits, err := ResolveLimits(v.GetString("limits"))
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve simulation limits: %w", err)
	}

	// Load experimental chains and group by family.
	expChains, err := settings.GetExperimentalChains(v)
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to load experimental chains config: %w", err)
	}

	expByFamily := make(map[string][]chainfamily.ExperimentalChainConfig)
	for _, ec := range expChains {
		expByFamily[ec.Family] = append(expByFamily[ec.Family], chainfamily.ExperimentalChainConfig{
			ChainSelector: ec.ChainSelector,
			RPCURL:        ec.RPCURL,
			Forwarder:     ec.Forwarder,
		})
	}

	// Set up each adapter.
	runtimes := make(map[string]chainfamily.ChainRuntime)
	selectorToRuntime := make(map[uint64]chainfamily.ChainRuntime)

	for _, adapter := range chainfamily.All() {
		family := adapter.Family()

		// Build RPC URL map for this adapter's supported chains only.
		rpcURLs := make(map[string]string)
		for _, chain := range adapter.SupportedChains() {
			chainName, err := settings.GetChainNameByChainSelector(chain.Selector)
			if err != nil {
				h.log.Debug().Msgf("Could not resolve chain name for selector %d; skipping", chain.Selector)
				continue
			}
			rpcURL, err := settings.GetRpcUrlSettings(v, chainName)
			if err != nil || strings.TrimSpace(rpcURL) == "" {
				continue
			}
			rpcURLs[chainName] = rpcURL
		}

		setupCfg := chainfamily.SetupConfig{
			Logger:             logger.Nop(),
			RPCURLs:            rpcURLs,
			ExperimentalChains: expByFamily[family],
			DryRun:             true, // default; adapters check their own broadcast flag
			SecretsPath:        creSettings.Workflow.WorkflowArtifactSettings.SecretsPath,
			FlagValues:         flagValues,
		}
		if simLimits != nil {
			setupCfg.ChainWriteReportSizeLimit = simLimits.ChainWriteReportSizeLimit()
			setupCfg.ChainWriteGasLimit = simLimits.ChainWriteEVMGasLimit()
		}

		rt, err := adapter.Setup(context.Background(), setupCfg)
		if err != nil {
			return Inputs{}, fmt.Errorf("failed to setup %s adapter: %w", family, err)
		}
		if rt == nil {
			// Adapter has no chains configured - skip.
			continue
		}

		runtimes[family] = rt
		for _, sel := range rt.OwnedSelectors() {
			selectorToRuntime[sel] = rt
		}
	}

	if len(runtimes) == 0 {
		return Inputs{}, fmt.Errorf("no chain adapters configured - check your RPC settings")
	}

	return Inputs{
		WasmPath:          v.GetString("wasm"),
		WorkflowPath:      creSettings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		ConfigPath:        cmdcommon.ResolveConfigPath(v, creSettings.Workflow.WorkflowArtifactSettings.ConfigPath),
		SecretsPath:       creSettings.Workflow.WorkflowArtifactSettings.SecretsPath,
		EngineLogs:        v.GetBool("engine-logs"),
		WorkflowName:      creSettings.Workflow.UserWorkflowSettings.WorkflowName,
		NonInteractive:    v.GetBool("non-interactive"),
		TriggerIndex:      v.GetInt("trigger-index"),
		HTTPPayload:       v.GetString("http-payload"),
		Runtimes:          runtimes,
		SelectorToRuntime: selectorToRuntime,
		FlagValues:        flagValues,
		LimitsPath:        v.GetString("limits"),
		SimLimits:         simLimits,
		SkipTypeChecks:    v.GetBool(cmdcommon.SkipTypeChecksCLIFlag),
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

	// Use limits already resolved during ResolveInputs.
	simLimits := inputs.SimLimits

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

		// Register chain-specific capabilities from each runtime
		for family, rt := range inputs.Runtimes {
			if err := rt.RegisterCapabilities(ctx, registry); err != nil {
				ui.Error(fmt.Sprintf("Failed to register %s capabilities: %v", family, err))
				os.Exit(1)
			}
		}

		// Register chain-agnostic trigger capabilities (cron + HTTP)
		triggerLggr := lggr.Named("TriggerCapabilities")
		triggerCaps, err = NewManualTriggerCapabilities(ctx, triggerLggr, registry)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create trigger capabilities: %v", err))
			os.Exit(1)
		}

		// Register chain-agnostic action capabilities (consensus, HTTP, conf HTTP)
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
		// Chain runtime services are NOT added to srvcs — their lifecycle is
		// managed by rt.Close() in simulatorCleanup to avoid double-close.
		srvcs = append(srvcs, computeCaps...)
		return registry, srvcs
	}

	// Create a holder for trigger info that will be populated in beforeStart
	holder := &triggerState{}

	getTriggerCaps := func() *ManualTriggers { return triggerCaps }
	var beforeStart func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription)
	if inputs.NonInteractive {
		beforeStart = makeBeforeStartNonInteractive(holder, inputs, getTriggerCaps)
	} else {
		beforeStart = makeBeforeStartInteractive(holder, inputs, getTriggerCaps)
	}

	waitFn := func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service) {
		<-initializedCh

		// Manual trigger execution
		if holder.TriggerFunc == nil {
			simLogger.Error("Trigger function not initialized")
			os.Exit(1)
		}
		if holder.TriggerToRun == nil {
			simLogger.Error("Trigger to run not selected")
			os.Exit(1)
		}
		simLogger.Info("Running trigger", "trigger", holder.TriggerToRun.GetId())
		err := holder.TriggerFunc()
		if err != nil {
			simLogger.Error("Failed to run trigger", "trigger", holder.TriggerToRun.GetId(), "error", err)
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

		// Close all chain runtimes
		for family, rt := range inputs.Runtimes {
			if err := rt.Close(); err != nil {
				simLogger.Error("Failed to close chain", "family", family, "error", err)
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
		BeforeStart: beforeStart,
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

type triggerState struct {
	TriggerFunc  func() error
	TriggerToRun *pb.TriggerSubscription
}

// buildChainTriggerFunc resolves the chain selector from a trigger subscription
// and delegates to the appropriate chain runtime to build the trigger function.
func buildChainTriggerFunc(ctx context.Context, trigger *pb.TriggerSubscription, triggerRegID string, inputs Inputs, promptUser bool) (func() error, error) {
	sel, ok := parseChainSelectorFromTriggerID(trigger.GetId())
	if !ok {
		return nil, fmt.Errorf("could not determine chain selector from trigger id %q", trigger.GetId())
	}

	rt, ok := inputs.SelectorToRuntime[sel]
	if !ok {
		return nil, fmt.Errorf("no chain adapter configured for chain selector %d; check your RPC settings", sel)
	}

	return rt.BuildTriggerFunc(ctx, chainfamily.TriggerRequest{
		TriggerID:             trigger.Id,
		TriggerRegistrationID: triggerRegID,
		ChainSelector:         sel,
		PromptUser:            promptUser,
		FlagValues:            inputs.FlagValues,
	})
}

// makeBeforeStartInteractive builds the interactive BeforeStart closure
func makeBeforeStartInteractive(holder *triggerState, inputs Inputs, triggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
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

		switch trigger {
		case "cron-trigger@1.0.0":
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, time.Now())
			}
		case "http-trigger@1.0.0-alpha":
			payload, err := getHTTPTriggerPayload()
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to get HTTP trigger payload: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		default:
			triggerFunc, err := buildChainTriggerFunc(ctx, holder.TriggerToRun, triggerRegistrationID, inputs, true)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to build trigger function: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = triggerFunc
		}
	}
}

// makeBeforeStartNonInteractive builds the non-interactive BeforeStart closure
func makeBeforeStartNonInteractive(holder *triggerState, inputs Inputs, triggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
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

		switch trigger {
		case "cron-trigger@1.0.0":
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, time.Now())
			}
		case "http-trigger@1.0.0-alpha":
			if strings.TrimSpace(inputs.HTTPPayload) == "" {
				ui.Error("--http-payload is required for http-trigger@1.0.0-alpha in non-interactive mode")
				os.Exit(1)
			}
			payload, err := getHTTPTriggerPayloadFromInput(inputs.HTTPPayload)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to parse HTTP trigger payload: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		default:
			triggerFunc, err := buildChainTriggerFunc(ctx, holder.TriggerToRun, triggerRegistrationID, inputs, false)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to build trigger function: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = triggerFunc
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

// getHTTPTriggerPayload prompts user for HTTP trigger data
func getHTTPTriggerPayload() (*httptypedapi.Payload, error) {
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

	// Check if input is a file path
	if _, err := os.Stat(input); err == nil {
		// It's a file path
		data, err := os.ReadFile(input)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", input, err)
		}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON from file %s: %w", input, err)
		}
		ui.Success(fmt.Sprintf("Loaded JSON from file: %s", input))
	} else {
		// It's direct JSON input
		if err := json.Unmarshal([]byte(input), &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		ui.Success("Parsed JSON input successfully")
	}

	jsonDataBytes, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	// Create the payload
	payload := &httptypedapi.Payload{
		Input: jsonDataBytes,
		// Key is optional for simulation
	}

	ui.Success(fmt.Sprintf("Created HTTP trigger payload with %d fields", len(jsonData)))
	return payload, nil
}

// getHTTPTriggerPayloadFromInput builds an HTTP trigger payload from a JSON string or a file path (optionally prefixed with '@')
func getHTTPTriggerPayloadFromInput(input string) (*httptypedapi.Payload, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("empty http payload input")
	}

	var raw []byte
	if strings.HasPrefix(trimmed, "@") {
		path := strings.TrimPrefix(trimmed, "@")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		raw = data
	} else {
		if _, err := os.Stat(trimmed); err == nil {
			data, err := os.ReadFile(trimmed)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", trimmed, err)
			}
			raw = data
		} else {
			raw = []byte(trimmed)
		}
	}

	return &httptypedapi.Payload{Input: raw}, nil
}
