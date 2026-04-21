//go:build wasip1

package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/aptos"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

// Config drives which aptos capability method the handler exercises.
//
// Scenario values:
//
//	balance                - AccountAPTBalance
//	view                   - View of coin::balance<AptosCoin>
//	tx-by-hash             - TransactionByHash (expect "not found" path)
//	account-transactions   - AccountTransactions pagination=1
type Config struct {
	Schedule      string `json:"schedule"`
	ChainSelector uint64 `json:"chain_selector"`
	Scenario      string `json:"scenario"`
	AddressHex    string `json:"address_hex"` // 32-byte hex, no 0x prefix
	TxHash        string `json:"tx_hash"`
}

func InitWorkflow(cfg *Config, _ *slog.Logger, _ cre.SecretsProvider) (cre.Workflow[*Config], error) {
	return cre.Workflow[*Config]{
		cre.Handler(cron.Trigger(&cron.Config{Schedule: cfg.Schedule}), runHandler),
	}, nil
}

func runHandler(cfg *Config, rt cre.Runtime, _ *cron.Payload) (string, error) {
	log := rt.Logger()
	client := &aptos.Client{ChainSelector: cfg.ChainSelector}

	addr, err := hex.DecodeString(cfg.AddressHex)
	if err != nil {
		return "", fmt.Errorf("bad address hex: %w", err)
	}

	switch cfg.Scenario {
	case "balance":
		reply, err := client.AccountAPTBalance(rt, &aptos.AccountAPTBalanceRequest{Address: addr}).Await()
		if err != nil {
			log.Info("aptos-smoke: balance failed", "err", err.Error())
			return "err:" + err.Error(), nil
		}
		log.Info("aptos-smoke: balance", "octas", reply.Value)
		return fmt.Sprintf("balance:%d", reply.Value), nil

	case "view":
		payload := &aptos.ViewRequest{
			Payload: &aptos.ViewPayload{
				Module:   &aptos.ModuleID{Address: aptosOneAddr(), Name: "coin"},
				Function: "balance",
				ArgTypes: nil,
				Args:     [][]byte{addr},
			},
		}
		reply, err := client.View(rt, payload).Await()
		if err != nil {
			log.Info("aptos-smoke: view failed", "err", err.Error())
			return "err:" + err.Error(), nil
		}
		log.Info("aptos-smoke: view", "bytes", len(reply.Data))
		return fmt.Sprintf("view:%d", len(reply.Data)), nil

	case "tx-by-hash":
		reply, err := client.TransactionByHash(rt, &aptos.TransactionByHashRequest{Hash: cfg.TxHash}).Await()
		if err != nil {
			log.Info("aptos-smoke: tx-by-hash failed", "err", err.Error())
			return "err:" + err.Error(), nil
		}
		if reply.Transaction == nil {
			log.Info("aptos-smoke: tx-by-hash missing")
			return "tx-by-hash:nil", nil
		}
		log.Info("aptos-smoke: tx-by-hash", "hash", reply.Transaction.Hash)
		return "tx-by-hash:" + reply.Transaction.Hash, nil

	case "account-transactions":
		var one uint64 = 1
		reply, err := client.AccountTransactions(rt, &aptos.AccountTransactionsRequest{
			Address: addr,
			Limit:   &one,
		}).Await()
		if err != nil {
			log.Info("aptos-smoke: account-transactions failed", "err", err.Error())
			return "err:" + err.Error(), nil
		}
		log.Info("aptos-smoke: account-transactions", "count", len(reply.Transactions))
		return fmt.Sprintf("account-transactions:%d", len(reply.Transactions)), nil
	}
	return "", fmt.Errorf("unknown scenario %q", cfg.Scenario)
}

// aptosOneAddr returns the 32-byte address 0x01 as required by coin module.
func aptosOneAddr() []byte {
	out := make([]byte, 32)
	out[31] = 0x01
	return out
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
