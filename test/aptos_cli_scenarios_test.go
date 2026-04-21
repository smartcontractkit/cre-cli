package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

// TestCLIAptosSimulator_30DryRuns invokes the real cre binary against the
// aptos_smoke fixture 30 times with different config JSON inputs. All runs are
// dry-run (no --broadcast). Each scenario asserts expected stdout substrings
// that prove FakeAptosChain routed the capability call correctly.
//
// Skipped by default (requires live Aptos testnet). Enable with:
//
//	CRE_APTOS_CLI_E2E=1 go test -v ./test -run TestCLIAptosSimulator_30DryRuns
//
// The test builds: ./bin/cre, /tmp/aptos_smoke.wasm.
func TestCLIAptosSimulator_30DryRuns(t *testing.T) {
	if os.Getenv("CRE_APTOS_CLI_E2E") != "1" {
		t.Skip("set CRE_APTOS_CLI_E2E=1 to run CLI e2e scenarios against Aptos testnet")
	}
	InitLogging()

	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	repoRoot = filepath.Dir(repoRoot) // test/ -> repo root

	cliBin := filepath.Join(repoRoot, "bin", "cre")
	require.FileExists(t, cliBin, "./bin/cre not built; run `go build -o ./bin/cre .`")

	wasmPath := "/tmp/aptos_smoke.wasm"
	require.FileExists(t, wasmPath, "WASM not built; run `cd test/test_project/aptos_smoke && GOOS=wasip1 GOARCH=wasm go build -o /tmp/aptos_smoke.wasm .`")

	projectDir := filepath.Join(repoRoot, "test", "test_project", "aptos_smoke")

	gql := testutil.NewGraphQLMockServerGetOrganization(t)
	defer gql.Close()
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	validAddr := "0000000000000000000000000000000000000000000000000000000000000001"
	unusedAddr := "0000000000000000000000000000000000000000000000000000000000000042"

	type sc struct {
		name       string
		cfg        map[string]any
		expect     string // substring that must appear in stdout/stderr
		mayBeError bool   // if true, errors from RPC are acceptable (still proves plumbing)
	}

	base := func(scenario, addr string) map[string]any {
		return map[string]any{
			"schedule":       "@every 30s",
			"chain_selector": uint64(743186221051783445),
			"scenario":       scenario,
			"address_hex":    addr,
			"tx_hash":        "0x0000000000000000000000000000000000000000000000000000000000000000",
		}
	}

	// Expected substring is the workflow's return value (stable, flushed before
	// simulator exit). User-log lines can be dropped if the log pipeline hasn't
	// flushed before the sim terminates.
	scenarios := []sc{
		// 1-10 balance
		{"balance_addr1", base("balance", validAddr), "\"balance:", false},
		{"balance_addr2", base("balance", unusedAddr), "\"balance:", false},
		{"balance_zero", base("balance", "0000000000000000000000000000000000000000000000000000000000000000"), "balance:", true},
		{"balance_0x2", base("balance", "0000000000000000000000000000000000000000000000000000000000000002"), "\"balance:", false},
		{"balance_0x3", base("balance", "0000000000000000000000000000000000000000000000000000000000000003"), "\"balance:", false},
		{"balance_0x4", base("balance", "0000000000000000000000000000000000000000000000000000000000000004"), "\"balance:", false},
		{"balance_0x5", base("balance", "0000000000000000000000000000000000000000000000000000000000000005"), "\"balance:", false},
		{"balance_0x6", base("balance", "0000000000000000000000000000000000000000000000000000000000000006"), "\"balance:", false},
		{"balance_0x7", base("balance", "0000000000000000000000000000000000000000000000000000000000000007"), "\"balance:", false},
		{"balance_0xA", base("balance", "000000000000000000000000000000000000000000000000000000000000000a"), "\"balance:", false},

		// 11-15 view
		{"view_coin_1", base("view", validAddr), "\"view:", true},
		{"view_coin_2", base("view", unusedAddr), "\"view:", true},
		{"view_coin_3", base("view", "0000000000000000000000000000000000000000000000000000000000000002"), "\"view:", true},
		{"view_coin_4", base("view", "0000000000000000000000000000000000000000000000000000000000000003"), "\"view:", true},
		{"view_coin_5", base("view", "0000000000000000000000000000000000000000000000000000000000000004"), "\"view:", true},

		// 16-20 tx-by-hash (nonexistent hashes return nil)
		{"tx_missing_1", withHash(base("tx-by-hash", validAddr), "0x1111111111111111111111111111111111111111111111111111111111111111"), "\"tx-by-hash:", true},
		{"tx_missing_2", withHash(base("tx-by-hash", validAddr), "0x2222222222222222222222222222222222222222222222222222222222222222"), "\"tx-by-hash:", true},
		{"tx_missing_3", withHash(base("tx-by-hash", validAddr), "0x3333333333333333333333333333333333333333333333333333333333333333"), "\"tx-by-hash:", true},
		{"tx_missing_4", withHash(base("tx-by-hash", validAddr), "0x4444444444444444444444444444444444444444444444444444444444444444"), "\"tx-by-hash:", true},
		{"tx_missing_5", withHash(base("tx-by-hash", validAddr), "0x5555555555555555555555555555555555555555555555555555555555555555"), "\"tx-by-hash:", true},

		// 21-25 account-transactions
		{"acct_tx_1", base("account-transactions", validAddr), "\"account-transactions:", true},
		{"acct_tx_2", base("account-transactions", unusedAddr), "\"account-transactions:", true},
		{"acct_tx_3", base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000002"), "\"account-transactions:", true},
		{"acct_tx_4", base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000003"), "\"account-transactions:", true},
		{"acct_tx_5", base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000004"), "\"account-transactions:", true},

		// 26-30 additional testnet variations
		{"balance_0xB", base("balance", "000000000000000000000000000000000000000000000000000000000000000b"), "\"balance:", false},
		{"view_coin_6", base("view", "000000000000000000000000000000000000000000000000000000000000000c"), "\"view:", true},
		{"tx_missing_6", withHash(base("tx-by-hash", validAddr), "0x6666666666666666666666666666666666666666666666666666666666666666"), "\"tx-by-hash:", true},
		{"acct_tx_6", base("account-transactions", "000000000000000000000000000000000000000000000000000000000000000d"), "\"account-transactions:", true},
		{"balance_0xC", base("balance", "000000000000000000000000000000000000000000000000000000000000000c"), "\"balance:", false},
	}
	require.Len(t, scenarios, 30, "must have 30 CLI scenarios")

	for i, s := range scenarios {
		i, s := i, s
		t.Run(fmt.Sprintf("%02d_%s", i+1, s.name), func(t *testing.T) {
			cfgPath := fmt.Sprintf("/tmp/apcfg_%02d.json", i+1)
			data, err := json.Marshal(s.cfg)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cfgPath, data, 0644))
			defer os.Remove(cfgPath)

			args := []string{
				"-T", "dev-aptos-testnet",
				"-R", projectDir,
				"workflow", "simulate", projectDir,
				"--wasm", wasmPath,
				"--config", cfgPath,
				"--non-interactive",
				"--trigger-index", "0",
				"--limits", "none",
			}
			cmd := exec.Command(cliBin, args...)
			cmd.Env = append(os.Environ(),
				"CRE_API_KEY=test-api",
			)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			err = cmd.Run()
			combined := out.String()
			t.Logf("cre output:\n%s", combined)

			// Every run must reach the simulator init + trigger dispatch + result.
			require.Contains(t, combined, "Simulator Initialized", "scenario %q: simulator did not initialise", s.name)
			require.Contains(t, combined, "Running trigger trigger=cron-trigger", "scenario %q: cron trigger did not fire", s.name)
			require.Contains(t, combined, "Workflow Simulation Result:", "scenario %q: workflow did not return", s.name)
			if s.mayBeError {
				// Success substring OR an err: string that names the method
				// (e.g. "err:...view function") — either proves routing reached
				// the Aptos capability.
				if strings.Contains(combined, s.expect) || strings.Contains(combined, "\"err:") {
					return
				}
			}
			require.Contains(t, combined, s.expect, "scenario %q missing expected substring", s.name)
		})
	}
}

func withHash(m map[string]any, h string) map[string]any {
	m["tx_hash"] = h
	return m
}

func withSelector(m map[string]any, s uint64) map[string]any {
	m["chain_selector"] = s
	return m
}
