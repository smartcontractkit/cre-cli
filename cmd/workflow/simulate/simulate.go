package simulate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	_ "github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain/evm" // register EVM chain family via package init
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const WorkflowExecutionTimeout = 5 * time.Minute

type Inputs struct {
	WasmPath     string `validate:"omitempty,file,ascii,max=97" cli:"--wasm"`
	WorkflowPath string `validate:"required,workflow_path_read"`
	ConfigPath   string `validate:"omitempty,file,ascii,max=97"`
	SecretsPath  string `validate:"omitempty,file,ascii,max=97"`
	EngineLogs   bool   `validate:"omitempty" cli:"--engine-logs"`
	Broadcast    bool   `validate:"-"`
	WorkflowName string `validate:"required"`
	// Chain-type-specific fields
	ChainTypeClients map[string]map[uint64]chain.ChainClient `validate:"omitempty"`
	ChainTypeKeys    map[string]interface{}                  `validate:"-"`
	// ChainTypeResolved holds the full ResolveClients bundle per chain type
	// (clients, forwarders, experimental-selector flags) so later steps
	// (health check, capability registration) have a single source of truth.
	ChainTypeResolved map[string]chain.ResolvedChains `validate:"-"`
	// Non-interactive mode options
	NonInteractive  bool              `validate:"-"`
	TriggerIndex    int               `validate:"-"`
	HTTPPayload     string            `validate:"-"` // JSON string or @/path/to/file.json
	ChainTypeInputs map[string]string `validate:"-"` // CLI-supplied chain-type-specific trigger inputs
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
	// Non-interactive flags
	simulateCmd.Flags().Bool(settings.Flags.NonInteractive.Name, false, "Run without prompts; requires --trigger-index and inputs for the selected trigger type")
	simulateCmd.Flags().Int("trigger-index", -1, "Index of the trigger to run (0-based)")
	simulateCmd.Flags().String("http-payload", "", "HTTP trigger payload as JSON string or path to JSON file (with or without @ prefix)")

	// Register chain-type-specific CLI flags (e.g., --evm-tx-hash).
	chain.RegisterAllCLIFlags(simulateCmd)

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
	chain.Build(h.log)

	ctClients := make(map[string]map[uint64]chain.ChainClient)
	ctResolved := make(map[string]chain.ResolvedChains)
	ctKeys := make(map[string]interface{})

	for name, ct := range chain.All() {
		resolved, err := ct.ResolveClients(v)
		if err != nil {
			return Inputs{}, fmt.Errorf("failed to resolve %s clients: %w", name, err)
		}

		if len(resolved.Clients) > 0 {
			ctClients[name] = resolved.Clients
			ctResolved[name] = resolved
		}
	}

	// Check at least one chain type has clients
	totalClients := 0
	for _, fc := range ctClients {
		totalClients += len(fc)
	}
	if totalClients == 0 {
		return Inputs{}, fmt.Errorf("no RPC URLs found for supported or experimental chains")
	}

	broadcast := v.GetBool("broadcast")
	for name, ct := range chain.All() {
		if _, ok := ctClients[name]; !ok {
			continue // no clients for this chain type; skip key resolution
		}
		key, err := ct.ResolveKey(creSettings, broadcast)
		if err != nil {
			return Inputs{}, err
		}
		if key != nil {
			ctKeys[name] = key
		}
	}

	return Inputs{
		WasmPath:          v.GetString("wasm"),
		WorkflowPath:      creSettings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		ConfigPath:        cmdcommon.ResolveConfigPath(v, creSettings.Workflow.WorkflowArtifactSettings.ConfigPath),
		SecretsPath:       creSettings.Workflow.WorkflowArtifactSettings.SecretsPath,
		EngineLogs:        v.GetBool("engine-logs"),
		Broadcast:         v.GetBool("broadcast"),
		ChainTypeClients:  ctClients,
		ChainTypeResolved: ctResolved,
		ChainTypeKeys:     ctKeys,
		WorkflowName:      creSettings.Workflow.UserWorkflowSettings.WorkflowName,
		NonInteractive:    v.GetBool("non-interactive"),
		TriggerIndex:      v.GetInt("trigger-index"),
		HTTPPayload:       v.GetString("http-payload"),
		ChainTypeInputs:   chain.CollectAllCLIInputs(v),
		LimitsPath:        v.GetString("limits"),
		SkipTypeChecks:    v.GetBool(cmdcommon.SkipTypeChecksCLIFlag),
		InvocationDir:     h.runtimeContext.InvocationDir,
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

	rpcErr := ui.WithSpinner("Checking RPC connectivity...", func() error {
		var errs []error
		for name, ct := range chain.All() {
			resolved, ok := inputs.ChainTypeResolved[name]
			if !ok {
				continue
			}
			if err := ct.RunHealthCheck(resolved); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("RPC health check failed:\n%w", errors.Join(errs...))
		}
		return nil
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

	var manualTriggerCaps *ManualTriggers
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

		// Register chain-agnostic cron and HTTP triggers
		triggerLggr := lggr.Named("TriggerCapabilities")
		var err error
		manualTriggerCaps, err = NewManualTriggerCapabilities(ctx, triggerLggr, registry)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create trigger capabilities: %v", err))
			os.Exit(1)
		}
		srvcs = append(srvcs, manualTriggerCaps.ManualCronTrigger, manualTriggerCaps.ManualHTTPTrigger)

		// Only set Limits when non-nil to avoid the typed-nil interface trap
		// (a nil *SimulationLimits boxed into chain.Limits compares != nil).
		var capLimits chain.Limits
		if simLimits != nil {
			capLimits = simLimits
		}

		// Register chain-type-specific capabilities
		for name, ct := range chain.All() {
			clients, ok := inputs.ChainTypeClients[name]
			if !ok || len(clients) == 0 {
				continue
			}

			ctSrvcs, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Registry:   registry,
				Clients:    clients,
				Forwarders: inputs.ChainTypeResolved[name].Forwarders,
				PrivateKey: inputs.ChainTypeKeys[name],
				Broadcast:  inputs.Broadcast,
				Limits:     capLimits,
				Logger:     triggerLggr,
			})
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to register %s capabilities: %v", name, err))
				os.Exit(1)
			}
			srvcs = append(srvcs, ctSrvcs...)
		}

		// Register chain-agnostic action capabilities (consensus, HTTP, confidential HTTP)
		computeLggr := lggr.Named("ActionsCapabilities")
		computeCaps, err := NewFakeActionCapabilities(ctx, computeLggr, registry, inputs.SecretsPath, simLimits)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create compute capabilities: %v", err))
			os.Exit(1)
		}

		// Start trigger capabilities
		if err := manualTriggerCaps.Start(ctx); err != nil {
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

		srvcs = append(srvcs, computeCaps...)
		return registry, srvcs
	}

	// Create a holder for trigger info that will be populated in beforeStart
	triggerInfoAndBeforeStart := &TriggerInfoAndBeforeStart{}

	getManualTriggerCaps := func() *ManualTriggers { return manualTriggerCaps }
	if inputs.NonInteractive {
		triggerInfoAndBeforeStart.BeforeStart = makeBeforeStartNonInteractive(triggerInfoAndBeforeStart, inputs, getManualTriggerCaps)
	} else {
		triggerInfoAndBeforeStart.BeforeStart = makeBeforeStartInteractive(triggerInfoAndBeforeStart, inputs, getManualTriggerCaps)
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
func makeBeforeStartInteractive(holder *TriggerInfoAndBeforeStart, inputs Inputs, manualTriggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
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
		manualTriggerCaps := manualTriggerCapsGetter()

		switch trigger {
		case "cron-trigger@1.0.0":
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

				return manualTriggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, skipWaitSignal)
			}
		case "http-trigger@1.0.0-alpha":
			payload, err := getHTTPTriggerPayload(inputs.InvocationDir)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to get HTTP trigger payload: %v", err))
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return manualTriggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		default:
			// Try each registered chain type
			handled := false
			for name, ct := range chain.All() {
				sel, ok := ct.ParseTriggerChainSelector(holder.TriggerToRun.GetId())
				if !ok {
					continue
				}

				if !ct.Supports(sel) {
					ui.Error(fmt.Sprintf("%s unsupported or misconfigured chain for selector %d", name, sel))
					os.Exit(1)
				}

				triggerData, err := getTriggerDataForChainType(ctx, ct, sel, inputs, true)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to get %s trigger data: %v", name, err))
					os.Exit(1)
				}

				handled = true
				holder.TriggerFunc = func() error {
					return ct.ExecuteTrigger(ctx, sel, triggerRegistrationID, triggerData)
				}
				break
			}

			if !handled {
				ui.Error(fmt.Sprintf("Unsupported trigger type: %s", holder.TriggerToRun.Id))
				os.Exit(1)
			}
		}
	}
}

// makeBeforeStartNonInteractive builds the non-interactive BeforeStart closure
func makeBeforeStartNonInteractive(holder *TriggerInfoAndBeforeStart, inputs Inputs, manualTriggerCapsGetter func() *ManualTriggers) func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service, []*pb.TriggerSubscription) {
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
		manualTriggerCaps := manualTriggerCapsGetter()

		switch trigger {
		case "cron-trigger@1.0.0":
			holder.TriggerFunc = func() error {
				skipWaitSignal := make(chan struct{}, 1)
				if err := manualTriggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, skipWaitSignal); err != nil {
					return err
				}
				// With cron schedule on non-interactive mode
				skipWaitSignal <- struct{}{}
				return nil
			}
		case "http-trigger@1.0.0-alpha":
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
				return manualTriggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		default:
			// Try each registered chain type
			handled := false
			for name, ct := range chain.All() {
				sel, ok := ct.ParseTriggerChainSelector(holder.TriggerToRun.GetId())
				if !ok {
					continue
				}

				if !ct.Supports(sel) {
					ui.Error(fmt.Sprintf("%s unsupported or misconfigured chain for selector %d", name, sel))
					os.Exit(1)
				}

				triggerData, err := getTriggerDataForChainType(ctx, ct, sel, inputs, false)
				if err != nil {
					ui.Error(fmt.Sprintf("Failed to get %s trigger data: %v", name, err))
					os.Exit(1)
				}

				handled = true
				holder.TriggerFunc = func() error {
					return ct.ExecuteTrigger(ctx, sel, triggerRegistrationID, triggerData)
				}
				break
			}

			if !handled {
				ui.Error(fmt.Sprintf("Unsupported trigger type: %s", holder.TriggerToRun.Id))
				os.Exit(1)
			}
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

// getHTTPTriggerPayload prompts user for HTTP trigger data. Relative paths are
// resolved against invocationDir so file references work from where the user ran
// the command even after SetExecutionContext switches cwd to the workflow dir.
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

// getTriggerDataForChainType resolves trigger data for a specific chain type.
// Each chain type defines its own trigger data format.
func getTriggerDataForChainType(ctx context.Context, ct chain.ChainType, selector uint64, inputs Inputs, interactive bool) (interface{}, error) {
	return ct.ResolveTriggerData(ctx, selector, chain.TriggerParams{
		Clients:         inputs.ChainTypeClients[ct.Name()],
		Interactive:     interactive,
		ChainTypeInputs: inputs.ChainTypeInputs,
	})
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
