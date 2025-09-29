//go:build wasip1

package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"t/contracts/evm/src/generated/balance_reader"
)

// EVMConfig holds per-chain configuration.
type EVMConfig struct {
	BalanceReaderAddress string `json:"balanceReaderAddress"`
	ChainSelector        uint64 `json:"chainSelector"`
	GasLimit             uint64 `json:"gasLimit"`
}

type Config struct {
	Schedule string      `json:"schedule"`
	EVMs     []EVMConfig `json:"evms"`
}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	cronTriggerCfg := &cron.Config{
		Schedule: config.Schedule,
	}

	return cre.Workflow[*Config]{
		cre.Handler(
			cron.Trigger(cronTriggerCfg),
			onPORCronTrigger,
		),
	}, nil
}

func onPORCronTrigger(config *Config, runtime cre.Runtime, outputs *cron.Payload) (string, error) {
	return doPOR(config, runtime)
}

func doPOR(config *Config, runtime cre.Runtime) (string, error) {
	logger := runtime.Logger()
	// Fetch PoR
	logger.Info("fetching por", "evms", config.EVMs)

	// use a default account which has 10000 ETH in the local evm chain
	secretAddressBalance, err := fetchNativeTokenBalance(runtime, config.EVMs[0], "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	if err != nil {
		return "", fmt.Errorf("failed to fetch secret address balance: %w", err)
	}
	logger.Info("Secret address balance", "balance", secretAddressBalance)

	return secretAddressBalance.String(), nil
}

func fetchNativeTokenBalance(runtime cre.Runtime, evmCfg EVMConfig, tokenHolderAddress string) (*big.Int, error) {
	logger := runtime.Logger()
	evmClient := &evm.Client{
		ChainSelector: evmCfg.ChainSelector,
	}

	balanceReaderAddress := common.HexToAddress(evmCfg.BalanceReaderAddress)
	balanceReader, err := balance_reader.NewBalanceReader(evmClient, balanceReaderAddress, nil)
	if err != nil {
		logger.Error("failed to create balance reader", "address", evmCfg.BalanceReaderAddress, "err", err)
		return nil, fmt.Errorf("failed to create balance reader for address %s: %w", evmCfg.BalanceReaderAddress, err)
	}
	tokenAddress, err := hexToBytes(tokenHolderAddress)
	if err != nil {
		logger.Error("failed to decode token address", "address", tokenHolderAddress, "err", err)
		return nil, fmt.Errorf("failed to decode token address %s: %w", tokenHolderAddress, err)
	}

	logger.Info("Getting native balances", "address", evmCfg.BalanceReaderAddress, "tokenAddress", tokenHolderAddress)
	balances, err := balanceReader.GetNativeBalances(runtime, balance_reader.GetNativeBalancesInput{
		Addresses: []common.Address{common.Address(tokenAddress)},
	}, big.NewInt(1)).Await()

	if err != nil {
		logger.Error("Could not read from contract", "contract_chain", evmCfg.ChainSelector, "err", err.Error())
		return nil, err
	}

	if len(balances) < 1 {
		logger.Error("No balances returned from contract", "contract_chain", evmCfg.ChainSelector)
		return nil, fmt.Errorf("no balances returned from contract for chain %d", evmCfg.ChainSelector)
	}

	return balances[0], nil
}

func hexToBytes(hexStr string) ([]byte, error) {
	if len(hexStr) < 2 || hexStr[:2] != "0x" {
		return nil, fmt.Errorf("invalid hex string: %s", hexStr)
	}
	return hex.DecodeString(hexStr[2:])
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
