//go:build wasip1

package main

import (
	"log/slog"
	"time"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

type EvmConfig struct {
	TokenAddress          string `json:"tokenAddress"`
	PorAddress            string `json:"porAddress"`
	ProxyAddress          string `json:"proxyAddress"`
	BalanceReaderAddress  string `json:"balanceReaderAddress"`
	MessageEmitterAddress string `json:"messageEmitterAddress"`
	ChainSelector         uint64 `json:"chainSelector"`
	GasLimit              uint64 `json:"gasLimit"`
}

type Config struct {
	Schedule string      `json:"schedule"`
	Url      string      `json:"url"`
	Evms     []EvmConfig `json:"evms"`
}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	cronTriggerCfg := &cron.Config{
		Schedule: config.Schedule,
	}

	return cre.Workflow[*Config]{
		cre.Handler(
			cron.Trigger(cronTriggerCfg),
			onPorCronTrigger),
	}, nil
}

func onPorCronTrigger(config *Config, runtime cre.Runtime, outputs *cron.Payload) (string, error) {
	return doPor(config, runtime, outputs.ScheduledExecutionTime.AsTime())
}

func doPor(config *Config, runtime cre.Runtime, _ time.Time) (string, error) {
	logger := runtime.Logger()
	logger.Info("assume the workflow is doing some stuff", "url", config.Url, "evms", config.Evms)

	return "1000", nil
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
