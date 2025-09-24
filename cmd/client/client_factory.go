package client

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

type Factory interface {
	NewCapabilitiesRegistryClient() (*CapabilitiesRegistryClient, error)
	NewWorkflowRegistryV2Client() (*WorkflowRegistryV2Client, error)
	GetTxType() TxType
	GetSkipConfirmation() bool
}

type factoryImpl struct {
	logger *zerolog.Logger
	viper  *viper.Viper
}

func NewFactory(logger *zerolog.Logger, viper *viper.Viper) Factory {
	return &factoryImpl{
		logger: logger,
		viper:  viper,
	}
}

func (f *factoryImpl) NewCapabilitiesRegistryClient() (*CapabilitiesRegistryClient, error) {
	environmentSet, err := environments.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment details: %w", err)
	}
	f.logger.Debug().
		Str("Address", environmentSet.CapabilitiesRegistryAddress).
		Str("Chain Name", environmentSet.CapabilitiesRegistryChainName).
		Msg("Selected Capabilities Registry")

	ethClient, err := f.newEthClient(environmentSet.CapabilitiesRegistryChainName)
	if err != nil {
		return nil, err
	}

	return NewCapabilitiesRegistryClient(f.logger, ethClient, common.HexToAddress(environmentSet.CapabilitiesRegistryAddress)), nil
}

func (f *factoryImpl) NewWorkflowRegistryV2Client() (*WorkflowRegistryV2Client, error) {
	environmentSet, err := environments.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment details: %w", err)
	}
	f.logger.Debug().
		Str("Address", environmentSet.WorkflowRegistryAddress).
		Str("Chain Name", environmentSet.WorkflowRegistryChainName).
		Msg("Selected Workflow Registry")

	ethClient, err := f.newEthClient(environmentSet.WorkflowRegistryChainName)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for chain %q: %w", environmentSet.WorkflowRegistryChainName, err)
	}

	txcConfig := TxClientConfig{
		TxType:       f.GetTxType(),
		LedgerConfig: f.getLedgerConfig(),
		SkipPrompt:   f.GetSkipConfirmation(),
	}

	workflowRegistryV2Client := NewWorkflowRegistryV2Client(
		f.logger,
		ethClient,
		environmentSet.WorkflowRegistryAddress,
		txcConfig,
	)

	typeAndVersion, err := workflowRegistryV2Client.TypeAndVersion()
	if err != nil {
		return workflowRegistryV2Client, fmt.Errorf("failed to get type and version of workflow registry contract at %s: %w", environmentSet.WorkflowRegistryAddress, err)
	}

	if !strings.Contains(typeAndVersion, constants.WorkflowRegistryV2TypeAndVersion) {
		return workflowRegistryV2Client, fmt.Errorf(
			"unexpected workflow registry type and version: expected to contain %q, got %q",
			constants.WorkflowRegistryV2TypeAndVersion, typeAndVersion,
		)
	}

	return workflowRegistryV2Client, nil
}

func (f *factoryImpl) newEthClient(chainName string) (*seth.Client, error) {
	wrRpcUrl, err := settings.GetRpcUrlSettings(f.viper, chainName)
	if err != nil {
		return nil, err
	}
	return NewEthClientFromEnv(f.viper, f.logger, wrRpcUrl)
}

func (f *factoryImpl) GetTxType() TxType {
	if f.viper.GetBool(settings.Flags.RawTxFlag.Name) {
		return Raw
	} else if f.viper.GetBool(settings.Flags.Ledger.Name) {
		return Ledger
	}
	return Regular
}

func (f *factoryImpl) GetSkipConfirmation() bool {
	return f.viper.GetBool(settings.Flags.SkipConfirmation.Name)
}

func (f *factoryImpl) getLedgerConfig() *LedgerConfig {
	ledgerEnabled := f.viper.GetBool(settings.Flags.Ledger.Name)
	derivationPath := f.viper.GetString(settings.Flags.LedgerDerivationPath.Name)
	if ledgerEnabled && derivationPath == "" {
		f.logger.Fatal().Msg("Ledger is enabled but no derivation path is provided")
	}
	return &LedgerConfig{
		LedgerEnabled:  ledgerEnabled,
		DerivationPath: derivationPath,
	}
}
