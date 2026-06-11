//go:build wasip1

package main

import (
	"fmt"
	"log/slog"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana"
	solanabindings "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana/bindings"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"

	"solana_log_trigger_workflow/contracts/solana/src/generated/data_storage"
)

// SolanaConfig holds per-chain log trigger settings.
type SolanaConfig struct {
	ChainName    string `json:"chainName"`
	FilterName   string `json:"filterName"`
	CallerFilter string `json:"callerFilter,omitempty"`
}

type Config struct {
	Solana SolanaConfig `json:"solana"`
}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	chainSelector, err := solana.ChainSelectorFromName(config.Solana.ChainName)
	if err != nil {
		return nil, fmt.Errorf("invalid chain name %q: %w", config.Solana.ChainName, err)
	}

	client := &solana.Client{ChainSelector: chainSelector}
	ds, err := data_storage.NewDataStorage(client)
	if err != nil {
		return nil, fmt.Errorf("create data storage bindings: %w", err)
	}

	filters, err := buildAccessLoggedFilters(config.Solana.CallerFilter)
	if err != nil {
		return nil, err
	}

	trigger, err := ds.LogTriggerAccessLoggedLog(
		chainSelector,
		config.Solana.FilterName,
		filters,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("register AccessLogged log trigger: %w", err)
	}

	return cre.Workflow[*Config]{
		cre.Handler(trigger, onAccessLogged),
	}, nil
}

func buildAccessLoggedFilters(callerBase58 string) ([]data_storage.AccessLoggedFilters, error) {
	if callerBase58 == "" {
		return nil, nil
	}

	caller, err := solanago.PublicKeyFromBase58(callerBase58)
	if err != nil {
		return nil, fmt.Errorf("invalid callerFilter pubkey %q: %w", callerBase58, err)
	}

	return []data_storage.AccessLoggedFilters{
		{Caller: &caller},
	}, nil
}

func onAccessLogged(
	config *Config,
	runtime cre.Runtime,
	payload *solanabindings.DecodedLog[data_storage.AccessLogged],
) (string, error) {
	runtime.Logger().Info(
		"AccessLogged event received",
		"filter", config.Solana.FilterName,
		"caller", payload.Data.Caller.String(),
		"message", payload.Data.Message,
	)

	return "ok", nil
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
