package simulate

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
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
	pb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	simulator "github.com/smartcontractkit/chainlink/v2/core/services/workflows/cmd/cre/utils"
	v2 "github.com/smartcontractkit/chainlink/v2/core/services/workflows/v2"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowPath  string                       `validate:"required,file"`
	ConfigPath    string                       `validate:"omitempty,file,ascii,max=97" cli:"--config"`
	SecretsPath   string                       `validate:"omitempty,file,ascii,max=97" cli:"--secrets"`
	EngineLogs    bool                         `validate:"omitempty" cli:"--engine-logs"`
	Broadcast     bool                         `validate:"-"`
	EVMClients    map[uint64]*ethclient.Client `validate:"omitempty"` // multichain clients keyed by selector
	EthPrivateKey *ecdsa.PrivateKey            `validate:"omitempty"`
	WorkflowName  string                       `validate:"required"`
	// Non-interactive mode options
	NonInteractive bool   `validate:"-"`
	TriggerIndex   int    `validate:"-"`
	HTTPPayload    string `validate:"-"` // JSON string or @/path/to/file.json
	EVMTxHash      string `validate:"-"` // 0x-prefixed
	EVMEventIndex  int    `validate:"-"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var simulateCmd = &cobra.Command{
		Use:   "simulate [workflow-name]",
		Short: "Simulates a workflow",
		Long:  `This command simulates a workflow.`,
		Args:  cobra.ExactArgs(1),
		Example: `
		cre workflow simulate my-workflow
		`,
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

	simulateCmd.Flags().BoolP("config", "c", false, "Should include a config file (path defined in the workflow settings file) (default: false)")
	simulateCmd.Flags().BoolP("secrets", "s", false, "Should include a secrets file (path defined in the workflow settings file) (default: false)")
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
	log       *zerolog.Logger
	validated bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:       ctx.Logger,
		validated: false,
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
			h.log.Info().Msgf("failed to create eth client for %s: %v", chainName, err)
			continue
		}

		clients[chain.Selector] = c
	}

	if len(clients) == 0 {
		return Inputs{}, fmt.Errorf("no RPC URLs found for supported chains")
	}

	pk, err := crypto.HexToECDSA(creSettings.User.EthPrivateKey)
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to get private key: %w", err)
	}

	return Inputs{
		WorkflowPath:   creSettings.Workflow.WorkflowArtifactSettings.WorkflowPath,
		ConfigPath:     creSettings.Workflow.WorkflowArtifactSettings.ConfigPath,
		SecretsPath:    creSettings.Workflow.WorkflowArtifactSettings.SecretsPath,
		EngineLogs:     v.GetBool("engine-logs"),
		Broadcast:      v.GetBool("broadcast"),
		EVMClients:     clients,
		EthPrivateKey:  pk,
		WorkflowName:   creSettings.Workflow.UserWorkflowSettings.WorkflowName,
		NonInteractive: v.GetBool("non-interactive"),
		TriggerIndex:   v.GetInt("trigger-index"),
		HTTPPayload:    v.GetString("http-payload"),
		EVMTxHash:      v.GetString("evm-tx-hash"),
		EVMEventIndex:  v.GetInt("evm-event-index"),
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

	if err := runRPCHealthCheck(inputs.EVMClients); err != nil {
		// we don't block execution, just show the error to the user
		// because some RPCs in settings might not be used in workflow and some RPCs might have hiccups
		h.log.Error().Msgf("some RPCs in setting is not functioning properly, please check: %v", err)
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
	isTypescriptWorkflow := strings.HasSuffix(workflowMainFile, ".ts")

	var buildCmd *exec.Cmd
	if isTypescriptWorkflow {
		buildCmd = exec.Command(
			"bun",
			"cre-compile",
			workflowMainFile,
			tmpWasmFileName,
		)
	} else {
		// The build command for reproducible and trimmed binaries.
		// -trimpath removes all file system paths from the compiled binary.
		// -ldflags="-buildid= -w -s" further reduces the binary size:
		//   -buildid= removes the build ID, ensuring reproducibility.
		//   -w disables DWARF debugging information.
		//   -s removes the symbol table.
		buildCmd = exec.Command(
			"go",
			"build",
			"-o", tmpWasmFileName,
			"-trimpath",
			"-ldflags=-buildid= -w -s",
			workflowMainFile,
		)
		buildCmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	}

	buildCmd.Dir = workflowRootFolder
	h.log.Debug().
		Str("Workflow directory", buildCmd.Dir).
		Str("Command", buildCmd.String()).
		Msg("Executing go build command")

	// Execute the build command
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		h.log.Info().Msg(string(buildOutput))
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	h.log.Debug().Msgf("Build output: %s", buildOutput)
	h.log.Info().Msg("Workflow compiled")

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

	return run(ctx, wasmFileBinary, config, secrets, inputs, verbosity)
}

// run instantiates the engine, starts it and blocks until the context is canceled.
func run(
	ctx context.Context,
	binary, config, secrets []byte,
	inputs Inputs,
	verbosity bool,
) error {
	logCfg := logger.Config{Level: getLevel(verbosity, zapcore.InfoLevel)}
	baseLggr, err := logCfg.New()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

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
				fmt.Printf("Failed to start billing service: %v\n", err)
				os.Exit(1)
			}

			srvcs = append(srvcs, bs)
		}

		if cfg.EnableBeholder {
			beholderLggr := lggr.Named("Beholder")
			err := setupCustomBeholder(beholderLggr, verbosity)
			if err != nil {
				fmt.Printf("Failed to setup beholder: %v\n", err)
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

		manualTriggerCapConfig := ManualTriggerCapabilitiesConfig{
			Clients:    inputs.EVMClients,
			PrivateKey: inputs.EthPrivateKey,
			Forwarders: forwarders,
		}

		triggerLggr := lggr.Named("TriggerCapabilities")
		triggerCaps, err = NewManualTriggerCapabilities(ctx, triggerLggr, registry, manualTriggerCapConfig, !inputs.Broadcast)
		if err != nil {
			fmt.Printf("failed to create trigger capabilities: %v\n", err)
			os.Exit(1)
		}

		computeLggr := lggr.Named("ActionsCapabilities")
		computeCaps, err := NewFakeActionCapabilities(ctx, computeLggr, registry)
		if err != nil {
			fmt.Printf("failed to create compute capabilities: %v\n", err)
			os.Exit(1)
		}

		// Start trigger capabilities
		if err := triggerCaps.Start(ctx); err != nil {
			fmt.Printf("failed to start trigger: %v\n", err)
			os.Exit(1)
		}

		// Start compute capabilities
		for _, cap := range computeCaps {
			if err = cap.Start(ctx); err != nil {
				fmt.Printf("failed to start capability: %v\n", err)
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
			baseLggr.Errorw("Trigger function not initialized")
			os.Exit(1)
		}
		if triggerInfoAndBeforeStart.TriggerToRun == nil {
			baseLggr.Errorw("Trigger to run not selected")
			os.Exit(1)
		}
		baseLggr.Infow("Running trigger", "trigger", triggerInfoAndBeforeStart.TriggerToRun.GetId())
		err := triggerInfoAndBeforeStart.TriggerFunc()
		if err != nil {
			baseLggr.Errorw("Failed to run trigger", "trigger", triggerInfoAndBeforeStart.TriggerToRun.GetId(), "error", err)
			os.Exit(1)
		}

		select {
		case <-executionFinishedCh:
			baseLggr.Infow("Execution finished signal received")
		case <-ctx.Done():
			baseLggr.Infow("Received interrupt signal, stopping execution")
		case <-time.After(WorkflowExecutionTimeout):
			baseLggr.Infow("Timeout waiting for execution to finish")
		}
	}
	simulatorCleanup := func(ctx context.Context, cfg simulator.RunnerConfig, registry *capabilities.Registry, services []services.Service) {
		for _, service := range services {
			if service.Name() == "WorkflowEngine.WorkflowEngineV2" {
				baseLggr.Info("Skipping WorkflowEngineV2")
				continue
			}

			if err := service.Close(); err != nil {
				baseLggr.Errorw("Failed to close service", "service", service.Name(), "error", err)
			}
		}

		err = cleanupBeholder()
		if err != nil {
			baseLggr.Warnw("Failed to cleanup beholder", "error", err)
		}
	}
	emptyHook := func(context.Context, simulator.RunnerConfig, *capabilities.Registry, []services.Service) {}

	// Ensure the workflow name is exactly 10 bytes before hex-encoding
	raw := []byte(inputs.WorkflowName)

	// Pad or truncate to exactly 10 bytes
	padded := make([]byte, 10)
	copy(padded, raw) // truncates if longer, zero-pads if shorter

	encodedWorkflowName := hex.EncodeToString(padded)

	simulator.NewRunner(&simulator.RunnerHooks{
		Initialize:  simulatorInitialize,
		BeforeStart: triggerInfoAndBeforeStart.BeforeStart,
		Wait:        waitFn,
		AfterRun:    emptyHook,
		Cleanup:     simulatorCleanup,
		Finally:     emptyHook,
	}).Run(ctx, encodedWorkflowName, binary, config, secrets, simulator.RunnerConfig{
		EnableBeholder: true,
		EnableBilling:  false,
		Lggr:           engineLog,
		LifecycleHooks: v2.LifecycleHooks{
			OnInitialized: func(err error) {
				if err != nil {
					baseLggr.Errorw("Failed to initialize simulator", "error", err)
					os.Exit(1)
				}
				baseLggr.Info("Simulator Initialized")
				fmt.Println()
				close(initializedCh)
			},
			OnExecutionError: func(msg string) {
				fmt.Println("Workflow execution failed:\n", msg)
				os.Exit(1)
			},
			OnResultReceived: func(result *pb.ExecutionResult) {
				fmt.Println()
				switch r := result.Result.(type) {
				case *pb.ExecutionResult_Value:
					v, err := values.FromProto(r.Value)
					if err != nil {
						fmt.Println("Could not decode result")
						break
					}

					uw, err := v.Unwrap()
					if err != nil {
						fmt.Printf("Could not unwrap result: %v", err)
						break
					}

					j, err := json.MarshalIndent(uw, "", "  ")
					if err != nil {
						fmt.Printf("Could not json marshal the result")
						break
					}

					fmt.Println("Workflow Simulation Result:\n", string(j))
				case *pb.ExecutionResult_Error:
					fmt.Println("Execution resulted in an error being returned: " + r.Error)
				}
				fmt.Println()
				close(executionFinishedCh)
			},
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
			fmt.Println("No triggers found")
			os.Exit(1)
		}

		var triggerIndex int
		if len(triggerSub) > 1 {
			// Present user with options and wait for selection
			fmt.Println("\n🚀 Workflow simulation ready. Please select a trigger:")
			for i, trigger := range triggerSub {
				fmt.Printf("%d. %s %s\n", i+1, trigger.GetId(), trigger.GetMethod())
			}
			fmt.Printf("\nEnter your choice (1-%d): ", len(triggerSub))

			holder.TriggerToRun, triggerIndex = getUserTriggerChoice(ctx, triggerSub)
			fmt.Println()
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
				fmt.Printf("failed to get HTTP trigger payload: %v\n", err)
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		case strings.HasPrefix(trigger, "evm") && strings.HasSuffix(trigger, "@1.0.0"):
			// Derive the chain selector directly from the selected trigger ID.
			sel, ok := parseChainSelectorFromTriggerID(holder.TriggerToRun.GetId())
			if !ok {
				fmt.Printf("could not determine chain selector from trigger id %q\n", holder.TriggerToRun.GetId())
				os.Exit(1)
			}

			client := inputs.EVMClients[sel]
			if client == nil {
				fmt.Printf("no RPC configured for chain selector %d\n", sel)
				os.Exit(1)
			}

			log, err := getEVMTriggerLog(ctx, client)
			if err != nil {
				fmt.Printf("failed to get EVM trigger log: %v\n", err)
				os.Exit(1)
			}
			evmChain := triggerCaps.ManualEVMChains[sel]
			if evmChain == nil {
				fmt.Printf("no EVM chain initialized for selector %d\n", sel)
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return evmChain.ManualTrigger(ctx, triggerRegistrationID, log)
			}
		default:
			fmt.Printf("unsupported trigger type: %s\n", holder.TriggerToRun.Id)
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
			fmt.Println("No triggers found")
			os.Exit(1)
		}
		if inputs.TriggerIndex < 0 {
			fmt.Println("--trigger-index is required when --non-interactive is enabled")
			os.Exit(1)
		}
		if inputs.TriggerIndex >= len(triggerSub) {
			fmt.Printf("invalid --trigger-index %d; available range: 0-%d\n", inputs.TriggerIndex, len(triggerSub)-1)
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
				fmt.Println("--http-payload is required for http-trigger@1.0.0-alpha in non-interactive mode")
				os.Exit(1)
			}
			payload, err := getHTTPTriggerPayloadFromInput(inputs.HTTPPayload)
			if err != nil {
				fmt.Printf("failed to parse HTTP trigger payload: %v\n", err)
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return triggerCaps.ManualHTTPTrigger.ManualTrigger(ctx, triggerRegistrationID, payload)
			}
		case strings.HasPrefix(trigger, "evm") && strings.HasSuffix(trigger, "@1.0.0"):
			if strings.TrimSpace(inputs.EVMTxHash) == "" || inputs.EVMEventIndex < 0 {
				fmt.Println("--evm-tx-hash and --evm-event-index are required for EVM triggers in non-interactive mode")
				os.Exit(1)
			}

			sel, ok := parseChainSelectorFromTriggerID(holder.TriggerToRun.GetId())
			if !ok {
				fmt.Printf("could not determine chain selector from trigger id %q\n", holder.TriggerToRun.GetId())
				os.Exit(1)
			}

			client := inputs.EVMClients[sel]
			if client == nil {
				fmt.Printf("no RPC configured for chain selector %d\n", sel)
				os.Exit(1)
			}

			log, err := getEVMTriggerLogFromValues(ctx, client, inputs.EVMTxHash, uint64(inputs.EVMEventIndex))
			if err != nil {
				fmt.Printf("failed to build EVM trigger log: %v\n", err)
				os.Exit(1)
			}
			evmChain := triggerCaps.ManualEVMChains[sel]
			if evmChain == nil {
				fmt.Printf("no EVM chain initialized for selector %d\n", sel)
				os.Exit(1)
			}
			holder.TriggerFunc = func() error {
				return evmChain.ManualTrigger(ctx, triggerRegistrationID, log)
			}
		default:
			fmt.Printf("unsupported trigger type: %s\n", holder.TriggerToRun.Id)
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
func setupCustomBeholder(lggr logger.Logger, verbosity bool) error {
	writer := &telemetryWriter{lggr: lggr, verbose: verbosity}

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

// getUserTriggerChoice handles user input for trigger selection
func getUserTriggerChoice(ctx context.Context, triggerSub []*pb.TriggerSubscription) (*pb.TriggerSubscription, int) {
	for {
		inputCh := make(chan string, 1)
		errCh := make(chan error, 1)

		go func() {
			// create a fresh reader for each attempt
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			inputCh <- input
		}()

		select {
		case <-ctx.Done():
			fmt.Println("\nReceived interrupt signal, exiting.")
			os.Exit(0)
		case err := <-errCh:
			fmt.Printf("Error reading input: %v\n", err)
			os.Exit(1)
		case input := <-inputCh:
			choice := strings.TrimSpace(input)
			choiceNum, err := strconv.Atoi(choice)
			if err != nil || choiceNum < 1 || choiceNum > len(triggerSub) {
				fmt.Printf("Invalid choice. Please enter 1-%d: ", len(triggerSub))
				continue
			}
			return triggerSub[choiceNum-1], (choiceNum - 1)
		}
	}
}

// getHTTPTriggerPayload prompts user for HTTP trigger data
func getHTTPTriggerPayload() (*httptypedapi.Payload, error) {
	fmt.Println("\n🔍 HTTP Trigger Configuration:")
	fmt.Println("Please provide JSON input for the HTTP trigger.")
	fmt.Println("You can enter a file path or JSON directly.")
	fmt.Print("\nEnter your input: ")

	// Create a fresh reader
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
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
		fmt.Printf("Loaded JSON from file: %s\n", input)
	} else {
		// It's direct JSON input
		if err := json.Unmarshal([]byte(input), &jsonData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		fmt.Println("Parsed JSON input successfully")
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

	fmt.Printf("Created HTTP trigger payload with %d fields\n", len(jsonData))
	return payload, nil
}

// getEVMTriggerLog prompts user for EVM trigger data and fetches the log
func getEVMTriggerLog(ctx context.Context, ethClient *ethclient.Client) (*evm.Log, error) {
	fmt.Println("\n🔗 EVM Trigger Configuration:")
	fmt.Println("Please provide the transaction hash and event index for the EVM log event.")

	// Create a fresh reader
	reader := bufio.NewReader(os.Stdin)

	// Get transaction hash
	fmt.Print("Enter transaction hash (0x...): ")
	txHashInput, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read transaction hash: %w", err)
	}
	txHashInput = strings.TrimSpace(txHashInput)

	if txHashInput == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}
	if !strings.HasPrefix(txHashInput, "0x") {
		return nil, fmt.Errorf("transaction hash must start with 0x")
	}
	if len(txHashInput) != 66 { // 0x + 64 hex chars
		return nil, fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(txHashInput))
	}

	txHash := common.HexToHash(txHashInput)

	// Get event index - create fresh reader
	fmt.Print("Enter event index (0-based): ")
	reader = bufio.NewReader(os.Stdin)
	eventIndexInput, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read event index: %w", err)
	}
	eventIndexInput = strings.TrimSpace(eventIndexInput)
	eventIndex, err := strconv.ParseUint(eventIndexInput, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid event index: %w", err)
	}

	// Fetch the transaction receipt
	fmt.Printf("Fetching transaction receipt for transaction %s...\n", txHash.Hex())
	txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}

	// Check if event index is valid
	if eventIndex >= uint64(len(txReceipt.Logs)) {
		return nil, fmt.Errorf("event index %d out of range, transaction has %d log events", eventIndex, len(txReceipt.Logs))
	}

	log := txReceipt.Logs[eventIndex]
	fmt.Printf("Found log event at index %d: contract=%s, topics=%d\n", eventIndex, log.Address.Hex(), len(log.Topics))

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

	fmt.Printf("Created EVM trigger log for transaction %s, event %d\n", txHash.Hex(), eventIndex)
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
	txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
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
