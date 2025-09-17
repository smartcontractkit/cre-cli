package client

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/environments"
	"github.com/smartcontractkit/dev-platform/internal/settings"
)

type Factory interface {
	NewCapabilitiesRegistryClient() (*CapabilitiesRegistryClient, error)
	NewWorkflowRegistryV2Client() (*WorkflowRegistryV2Client, error)
	GetTxType() TxType
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
		Uint64("Chain Selector", environmentSet.CapabilitiesRegistryChainSelector).
		Msg("Selected Capabilities Registry")

	ethClient, err := f.newEthClient(environmentSet.CapabilitiesRegistryChainSelector)
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
		Uint64("Chain Selector", environmentSet.WorkflowRegistryChainSelector).
		Msg("Selected Workflow Registry")

	ethClient, err := f.newEthClient(environmentSet.WorkflowRegistryChainSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for chain selector %d: %w", environmentSet.WorkflowRegistryChainSelector, err)
	}

	workflowRegistryV2Client := NewWorkflowRegistryV2Client(
		f.logger,
		ethClient,
		environmentSet.WorkflowRegistryAddress,
		f.GetTxType(),
		f.getLedgerConfig(),
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

func (f *factoryImpl) newEthClient(chainSelector uint64) (*seth.Client, error) {
	wrRpcUrl, err := settings.GetRpcUrlSettings(f.viper, chainSelector)
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
