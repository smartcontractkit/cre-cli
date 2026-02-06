package simulate

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
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

	"github.com/charmbracelet/huh"
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
	WorkflowPath  string                       `validate:"required,path_read"`
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
	// Non-interactive flags
	simulateCmd.Flags().Bool(settings.Flags.NonInteractive.Name, false, "Run without prompts; requires --trigger-index and inputs for the selected trigger type")
	simulateCmd.Flags().Int("trigger-index", -1, "Index of the trigger to run (0-based)")
	simulateCmd.Flags().String("http-payload", "", "HTTP trigger payload as JSON string or path to JSON file (with or without @ prefix)")
	simulateCmd.Flags().String("evm-tx-hash", "", "EVM trigger transaction hash (0x...)")
	simulateCmd.Flags().Int("evm-event-index", -1, "EVM trigger log index (0-based)")
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
		if ec.ChainID == 0 {
			return Inputs{}, fmt.Errorf("experimental chain missing chain-id")
		}
		if strings.TrimSpace(ec.RPCURL) == "" {
			return Inputs{}, fmt.Errorf("experimental chain %d missing rpc-url", ec.ChainID)
		}
		if strings.TrimSpace(ec.Forwarder) == "" {
			return Inputs{}, fmt.Errorf("experimental chain %d missing forwarder", ec.ChainID)
		}

		// Check if chain ID already exists (supported chain)
		if _, exists := clients[ec.ChainID]; exists {
			// Find the supported chain's forwarder
			var supportedForwarder string
			for _, supported := range SupportedEVM {
				if supported.Selector == ec.ChainID {
					supportedForwarder = supported.Forwarder
					break
				}
			}

			expFwd := common.HexToAddress(ec.Forwarder)
			if supportedForwarder != "" && common.HexToAddress(supportedForwarder) == expFwd {
				// Same forwarder, just debug log
				h.log.Debug().Uint64("chain-id", ec.ChainID).Msg("Experimental chain matches supported chain config")
				continue
			}

			// Different forwarder - respect user's config, warn about override
			ui.Warning(fmt.Sprintf("Experimental chain %d overrides supported chain forwarder (supported: %s, experimental: %s)", ec.ChainID, supportedForwarder, ec.Forwarder))

			// Use existing client but override the forwarder
			experimentalForwarders[ec.ChainID] = expFwd
			continue
		}

		// Dial the RPC
		c, err := ethclient.Dial(ec.RPCURL)
		if err != nil {
			return Inputs{}, fmt.Errorf("failed to create eth client for experimental chain %d: %w", ec.ChainID, err)
		}

		clients[ec.ChainID] = c
		experimentalForwarders[ec.ChainID] = common.HexToAddress(ec.Forwarder)
		ui.Dim(fmt.Sprintf("Added experimental chain (chain-id: %d)", ec.ChainID))
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
		WorkflowPath:           creSettings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		ConfigPath:             creSettings.Workflow.WorkflowArtifactSettings.ConfigPath,
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
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err = validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	// forbid the default 0x...01 key when broadcasting
	if inputs.Broadcast && inputs.EthPrivateKey != nil && inputs.EthPrivateKey.D.Cmp(big.NewInt(1)) == 0 {
		return fmt.Errorf("you must configure a valid private key to perform on-chain writes. Please set your private key in the .env file before using the -–broadcast flag")
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
	// Compile the workflow
	// terminal command: GOOS=wasip1 GOARCH=wasm go build -trimpath -ldflags="-buildid= -w -s" -o <output_path> <workflow_path>
	workflowRootFolder := filepath.Dir(inputs.WorkflowPath)
	tmpWasmFileName := "tmp.wasm"
	workflowMainFile := filepath.Base(inputs.WorkflowPath)

	// Set language in runtime context based on workflow file extension
	if h.runtimeContext != nil {
		h.runtimeContext.Workflow.Language = cmdcommon.GetWorkflowLanguage(workflowMainFile)

		switch h.runtimeContext.Workflow.Language {
		case constants.WorkflowLanguageTypeScript:
			if err := cmdcommon.EnsureTool("bun"); err != nil {
				return errors.New("bun is required for TypeScript workflows but was not found in PATH; install from https://bun.com/docs/installation")
			}
		case constants.WorkflowLanguageGolang:
			if err := cmdcommon.EnsureTool("go"); err != nil {
				return errors.New("go toolchain is required for Go workflows but was not found in PATH; install from https://go.dev/dl")
			}
		default:
			return fmt.Errorf("unsupported workflow language for file %s", workflowMainFile)
		}
	}

	buildCmd := cmdcommon.GetBuildCmd(workflowMainFile, tmpWasmFileName, workflowRootFolder)

	h.log.Debug().
		Str("Workflow directory", buildCmd.Dir).
		Str("Command", buildCmd.String()).
		Msg("Executing go build command")

	// Execute the build command with spinner
	spinner := ui.NewSpinner()
	spinner.Start("Compiling workflow...")
	buildOutput, err := buildCmd.CombinedOutput()
	spinner.Stop()

	if err != nil {
		out := strings.TrimSpace(string(buildOutput))
		h.log.Info().Msg(out)
		return fmt.Errorf("failed to compile workflow: %w\nbuild output:\n%s", err, out)
	}
	h.log.Debug().Msgf("Build output: %s", buildOutput)
	ui.Success("Workflow compiled")

	// Read the compiled workflow binary
	tmpWasmLocation := filepath.Join(workflowRootFolder, tmpWasmFileName)
	wasmFileBinary, err := os.ReadFile(tmpWasmLocation)
	if err != nil {
		return fmt.Errorf("failed to read workflow binary: %w", err)
	}

	// Read the config file
	var config []byte
	if inputs.ConfigPath != "" {
		config, err = os.ReadFile(inputs.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

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

	err = run(ctx, wasmFileBinary, config, secrets, inputs, verbosity)
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
		triggerCaps, err = NewManualTriggerCapabilities(ctx, triggerLggr, registry, manualTriggerCapConfig, !inputs.Broadcast)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to create trigger capabilities: %v", err))
			os.Exit(1)
		}

		computeLggr := lggr.Named("ActionsCapabilities")
		computeCaps, err := NewFakeActionCapabilities(ctx, computeLggr, registry)
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
			cfg.ChainAllowed = commonsettings.PerChainSelector(
				commonsettings.Bool(true), // Allow all chains in simulation
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
			// Build options for huh select
			options := make([]huh.Option[int], len(triggerSub))
			for i, trigger := range triggerSub {
				options[i] = huh.NewOption(fmt.Sprintf("%s %s", trigger.GetId(), trigger.GetMethod()), i)
			}

			ui.Line()
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[int]().
						Title("Workflow simulation ready. Please select a trigger:").
						Options(options...).
						Value(&triggerIndex),
				),
			).WithTheme(ui.ChainlinkTheme())

			if err := form.Run(); err != nil {
				ui.Error(fmt.Sprintf("Trigger selection failed: %v", err))
				os.Exit(1)
			}

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
				return triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, time.Now())
			}
		case trigger == "http-trigger@1.0.0-alpha":
			payload, err := getHTTPTriggerPayload()
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
				return triggerCaps.ManualCronTrigger.ManualTrigger(ctx, triggerRegistrationID, time.Now())
			}
		case trigger == "http-trigger@1.0.0-alpha":
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

			log, err := getEVMTriggerLogFromValues(ctx, client, inputs.EVMTxHash, uint64(inputs.EVMEventIndex))
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

// getHTTPTriggerPayload prompts user for HTTP trigger data
func getHTTPTriggerPayload() (*httptypedapi.Payload, error) {
	var input string

	ui.Line()
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("HTTP Trigger Configuration").
				Description("Enter a file path or JSON directly for the HTTP trigger").
				Placeholder(`{"key": "value"} or ./payload.json`).
				Value(&input),
		),
	).WithTheme(ui.ChainlinkTheme())

	if err := form.Run(); err != nil {
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

// getEVMTriggerLog prompts user for EVM trigger data and fetches the log
func getEVMTriggerLog(ctx context.Context, ethClient *ethclient.Client) (*evm.Log, error) {
	var txHashInput string
	var eventIndexInput string

	ui.Line()
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("EVM Trigger Configuration").
				Description("Transaction hash for the EVM log event").
				Placeholder("0x...").
				Value(&txHashInput).
				Validate(func(s string) error {
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
				}),
			huh.NewInput().
				Title("Event Index").
				Description("Log event index (0-based)").
				Placeholder("0").
				Suggestions([]string{"0"}).
				Value(&eventIndexInput).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("event index cannot be empty")
					}
					if _, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32); err != nil {
						return fmt.Errorf("invalid event index: must be a number")
					}
					return nil
				}),
		),
	).WithTheme(ui.ChainlinkTheme()).WithKeyMap(ui.ChainlinkKeyMap())

	if err := form.Run(); err != nil {
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

	//var jsonData map[string]interface{}
	//if err := json.Unmarshal(raw, &jsonData); err != nil {
	//	return nil, fmt.Errorf("failed to parse JSON: %w", err)
	//}

	//structPB, err := structpb.NewStruct(jsonData)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to convert to protobuf struct: %w", err)
	//}

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
