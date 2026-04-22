package hash

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	workflowUtils "github.com/smartcontractkit/chainlink-common/pkg/workflows"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/runtimeattach"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type Inputs struct {
	ForUser           string
	WasmPath          string
	ConfigPath        string
	WorkflowName      string
	WorkflowPath      string
	OwnerFromSettings string
	PrivateKey        string
	SkipTypeChecks    bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	hashCmd := &cobra.Command{
		Use:   "hash <workflow-folder-path>",
		Short: "Computes and displays workflow hashes",
		Long:  `Computes the binary hash, config hash, and workflow hash for a workflow. The workflow hash uses the same algorithm as the on-chain workflow ID.`,
		Args:  cobra.ExactArgs(1),
		Example: `  cre workflow hash ./my-workflow
  cre workflow hash ./my-workflow --public_key 0x1234...abcd`,
		RunE: func(cmd *cobra.Command, args []string) error {
			forUser, _ := cmd.Flags().GetString("public_key")

			s := runtimeContext.Settings
			v := runtimeContext.Viper

			rawPrivKey := v.GetString(settings.EthPrivateKeyEnvVar)

			inputs := Inputs{
				ForUser:           forUser,
				WasmPath:          v.GetString("wasm"),
				ConfigPath:        cmdcommon.ResolveConfigPath(v, s.Workflow.WorkflowArtifactSettings.ConfigPath),
				WorkflowName:      s.Workflow.UserWorkflowSettings.WorkflowName,
				WorkflowPath:      s.Workflow.WorkflowArtifactSettings.WorkflowPath,
				OwnerFromSettings: s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
				PrivateKey:        settings.NormalizeHexKey(rawPrivKey),
				SkipTypeChecks:    v.GetBool(cmdcommon.SkipTypeChecksCLIFlag),
			}

			return Execute(inputs)
		},
	}

	hashCmd.Flags().String("public_key", "",
		"Owner address to use for computing the workflow hash. "+
			"Required when CRE_ETH_PRIVATE_KEY is not set and no workflow-owner-address is configured. "+
			"Defaults to the address derived from CRE_ETH_PRIVATE_KEY or the workflow-owner-address in project settings.")
	hashCmd.Flags().String("wasm", "", "Path or URL to a pre-built WASM binary (skips compilation)")
	hashCmd.Flags().String("config", "", "Override the config file path from workflow.yaml")
	hashCmd.Flags().Bool("no-config", false, "Hash without a config file")
	hashCmd.Flags().Bool("default-config", false, "Use the config path from workflow.yaml settings (default behavior)")
	hashCmd.MarkFlagsMutuallyExclusive("config", "no-config", "default-config")
	hashCmd.Flags().Bool(cmdcommon.SkipTypeChecksCLIFlag, false, "Skip TypeScript project typecheck during compilation (passes "+cmdcommon.SkipTypeChecksFlag+" to cre-compile)")

	runtimeattach.Register(hashCmd, runtimeattach.ProjectSettingsNoCreds)
	return hashCmd
}

func Execute(inputs Inputs) error {
	rawBinary, err := loadBinary(inputs.WasmPath, inputs.WorkflowPath, inputs.SkipTypeChecks)
	if err != nil {
		return err
	}

	binary, err := cmdcommon.CompressBrotli(rawBinary)
	if err != nil {
		return fmt.Errorf("failed to compress binary: %w", err)
	}

	config, err := loadConfig(inputs.ConfigPath)
	if err != nil {
		return err
	}

	ownerAddress, err := ResolveOwner(inputs.ForUser, inputs.OwnerFromSettings, inputs.PrivateKey)
	if err != nil {
		return err
	}

	binaryHash := cmdcommon.HashBytes(binary)
	configHash := cmdcommon.HashBytes(config)

	workflowID, err := workflowUtils.GenerateWorkflowIDFromStrings(ownerAddress, inputs.WorkflowName, binary, config, "")
	if err != nil {
		return fmt.Errorf("failed to generate workflow hash: %w", err)
	}

	ui.Dim(fmt.Sprintf("Binary hash:   %s", binaryHash))
	ui.Dim(fmt.Sprintf("Config hash:   %s", configHash))
	ui.Dim(fmt.Sprintf("Workflow hash: %s", workflowID))

	return nil
}

func ResolveOwner(forUser, ownerFromSettings, privateKey string) (string, error) {
	if forUser != "" {
		return forUser, nil
	}

	if ownerFromSettings != "" {
		return ownerFromSettings, nil
	}

	if privateKey != "" {
		addr, err := ethkeys.DeriveEthAddressFromPrivateKey(privateKey)
		if err != nil {
			return "", fmt.Errorf("failed to derive owner from private key: %w", err)
		}
		return addr, nil
	}

	return "", fmt.Errorf("cannot determine workflow owner: provide --public_key or ensure CRE_ETH_PRIVATE_KEY is set")
}

func loadBinary(wasmFlag, workflowPathFromSettings string, skipTypeChecks bool) ([]byte, error) {
	if wasmFlag != "" {
		if cmdcommon.IsURL(wasmFlag) {
			ui.Dim("Fetching WASM binary from URL...")
			data, err := cmdcommon.FetchURL(wasmFlag)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch WASM from URL: %w", err)
			}
			ui.Success("Fetched WASM binary from URL")
			return cmdcommon.EnsureRawWasm(data)
		}
		ui.Dim("Reading pre-built WASM binary...")
		data, err := os.ReadFile(wasmFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to read WASM binary: %w", err)
		}
		ui.Success(fmt.Sprintf("Loaded WASM binary from %s", wasmFlag))
		return cmdcommon.EnsureRawWasm(data)
	}

	workflowDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("workflow directory: %w", err)
	}
	resolvedWorkflowPath, err := cmdcommon.ResolveWorkflowPath(workflowDir, workflowPathFromSettings)
	if err != nil {
		return nil, fmt.Errorf("workflow path: %w", err)
	}

	spinner := ui.NewSpinner()
	spinner.Start("Compiling workflow...")
	wasmBytes, err := cmdcommon.CompileWorkflowToWasm(resolvedWorkflowPath, cmdcommon.WorkflowCompileOptions{
		StripSymbols:   true,
		SkipTypeChecks: skipTypeChecks,
	})
	spinner.Stop()
	if err != nil {
		ui.Error("Build failed:")
		return nil, fmt.Errorf("failed to compile workflow: %w", err)
	}
	ui.Success("Workflow compiled")

	return wasmBytes, nil
}

func loadConfig(configPath string) ([]byte, error) {
	if configPath == "" {
		return nil, nil
	}
	if cmdcommon.IsURL(configPath) {
		ui.Dim("Fetching config from URL...")
		data, err := cmdcommon.FetchURL(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch config from URL: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return data, nil
}
