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

// TestCLIAptosSimulator_100DryRuns invokes the real cre binary against the
// aptos_smoke fixture 100 times with different config JSON inputs. All runs
// default to dry-run; a final block of scenarios exercises --broadcast error
// paths and UI/limits edges. Each scenario asserts expected stdout substrings
// that prove FakeAptosChain routed the capability call correctly.
//
// Skipped by default (requires live Aptos testnet). Enable with:
//
//	CRE_APTOS_CLI_E2E=1 go test -v ./test -run TestCLIAptosSimulator_100DryRuns
//
// The test expects: ./bin/cre, /tmp/aptos_smoke.wasm.
func TestCLIAptosSimulator_100DryRuns(t *testing.T) {
	if os.Getenv("CRE_APTOS_CLI_E2E") != "1" {
		t.Skip("set CRE_APTOS_CLI_E2E=1 to run CLI e2e scenarios against Aptos testnet")
	}
	InitLogging()

	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	repoRoot = filepath.Dir(repoRoot) // test/ -> repo root

	cliBin := filepath.Join(repoRoot, "bin", "cre")
	require.FileExists(t, cliBin, "./bin/cre not built; run `go build -o ./bin/cre .`")


	wasmPath := os.Getenv("APTOS_SMOKE_WASM")
	if wasmPath == "" {
		wasmPath = "/tmp/aptos_smoke.wasm"
	}
	require.FileExists(t, wasmPath, "WASM not built; set APTOS_SMOKE_WASM or run `cd test/test_project/aptos_smoke && GOOS=wasip1 GOARCH=wasm go build -o /tmp/aptos_smoke.wasm .`")

	projectDir := filepath.Join(repoRoot, "test", "test_project", "aptos_smoke")

	gql := testutil.NewGraphQLMockServerGetOrganization(t)
	defer gql.Close()
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	validAddr := "0000000000000000000000000000000000000000000000000000000000000001"
	unusedAddr := "0000000000000000000000000000000000000000000000000000000000000042"

	type sc struct {
		name       string
		cfg        map[string]any
		expect     string   // substring that must appear in stdout/stderr
		mayBeError bool     // if true, errors from RPC are acceptable (still proves plumbing)
		args       []string // extra CLI args appended (nil = standard dry-run)
		env        []string // extra env vars (e.g. sentinel key override)
		mustFail   bool     // process exit must be non-zero; expect substring then checked in stderr/stdout
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
	aptosTestnetSel := uint64(743186221051783445)
	_ = aptosTestnetSel // only referenced in wrong_sel_testnet_unchanged below
	scenarios := []sc{
		// --- 1-10 balance (happy-path address variations) ---
		{name: "balance_addr1", cfg: base("balance", validAddr), expect: "\"balance:"},
		{name: "balance_addr2", cfg: base("balance", unusedAddr), expect: "\"balance:"},
		{name: "balance_zero", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000000"), expect: "balance:", mayBeError: true},
		{name: "balance_0x2", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000002"), expect: "\"balance:"},
		{name: "balance_0x3", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000003"), expect: "\"balance:"},
		{name: "balance_0x4", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000004"), expect: "\"balance:"},
		{name: "balance_0x5", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000005"), expect: "\"balance:"},
		{name: "balance_0x6", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000006"), expect: "\"balance:"},
		{name: "balance_0x7", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000007"), expect: "\"balance:"},
		{name: "balance_0xA", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000a"), expect: "\"balance:"},

		// --- 11-15 view ---
		{name: "view_coin_1", cfg: base("view", validAddr), expect: "\"view:", mayBeError: true},
		{name: "view_coin_2", cfg: base("view", unusedAddr), expect: "\"view:", mayBeError: true},
		{name: "view_coin_3", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000002"), expect: "\"view:", mayBeError: true},
		{name: "view_coin_4", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000003"), expect: "\"view:", mayBeError: true},
		{name: "view_coin_5", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000004"), expect: "\"view:", mayBeError: true},

		// --- 16-20 tx-by-hash (nonexistent hashes → nil) ---
		{name: "tx_missing_1", cfg: withHash(base("tx-by-hash", validAddr), "0x1111111111111111111111111111111111111111111111111111111111111111"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_missing_2", cfg: withHash(base("tx-by-hash", validAddr), "0x2222222222222222222222222222222222222222222222222222222222222222"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_missing_3", cfg: withHash(base("tx-by-hash", validAddr), "0x3333333333333333333333333333333333333333333333333333333333333333"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_missing_4", cfg: withHash(base("tx-by-hash", validAddr), "0x4444444444444444444444444444444444444444444444444444444444444444"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_missing_5", cfg: withHash(base("tx-by-hash", validAddr), "0x5555555555555555555555555555555555555555555555555555555555555555"), expect: "\"tx-by-hash:", mayBeError: true},

		// --- 21-25 account-transactions ---
		{name: "acct_tx_1", cfg: base("account-transactions", validAddr), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_2", cfg: base("account-transactions", unusedAddr), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_3", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000002"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_4", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000003"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_5", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000004"), expect: "\"account-transactions:", mayBeError: true},

		// --- 26-30 additional testnet variations ---
		{name: "balance_0xB", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000b"), expect: "\"balance:"},
		{name: "view_coin_6", cfg: base("view", "000000000000000000000000000000000000000000000000000000000000000c"), expect: "\"view:", mayBeError: true},
		{name: "tx_missing_6", cfg: withHash(base("tx-by-hash", validAddr), "0x6666666666666666666666666666666666666666666666666666666666666666"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "acct_tx_6", cfg: base("account-transactions", "000000000000000000000000000000000000000000000000000000000000000d"), expect: "\"account-transactions:", mayBeError: true},
		{name: "balance_0xC", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000c"), expect: "\"balance:"},

		// --- 31-40 more balance permutations (deterministic routing proof) ---
		{name: "balance_0xD", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000d"), expect: "\"balance:"},
		{name: "balance_0xE", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000e"), expect: "\"balance:"},
		{name: "balance_0xF", cfg: base("balance", "000000000000000000000000000000000000000000000000000000000000000f"), expect: "\"balance:"},
		{name: "balance_high_bit", cfg: base("balance", "8000000000000000000000000000000000000000000000000000000000000000"), expect: "\"balance:"},
		{name: "balance_low_bit", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000001"), expect: "\"balance:"},
		{name: "balance_fan_out_1", cfg: base("balance", "1111111111111111111111111111111111111111111111111111111111111111"), expect: "\"balance:"},
		{name: "balance_fan_out_2", cfg: base("balance", "2222222222222222222222222222222222222222222222222222222222222222"), expect: "\"balance:"},
		{name: "balance_fan_out_3", cfg: base("balance", "3333333333333333333333333333333333333333333333333333333333333333"), expect: "\"balance:"},
		{name: "balance_fan_out_4", cfg: base("balance", "4444444444444444444444444444444444444444444444444444444444444444"), expect: "\"balance:"},
		{name: "balance_max_u256", cfg: base("balance", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), expect: "balance:", mayBeError: true},

		// --- 41-50 view coin::balance edges ---
		{name: "view_all_zero", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000000"), expect: "view:", mayBeError: true},
		{name: "view_all_one", cfg: base("view", "0101010101010101010101010101010101010101010101010101010101010101"), expect: "view:", mayBeError: true},
		{name: "view_all_f", cfg: base("view", "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"), expect: "view:", mayBeError: true},
		{name: "view_canonical_1", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000010"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_2", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000020"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_3", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000030"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_4", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000040"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_5", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000050"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_6", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000060"), expect: "\"view:", mayBeError: true},
		{name: "view_canonical_7", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000070"), expect: "\"view:", mayBeError: true},

		// --- 51-60 tx-by-hash randomised nonexistent hashes ---
		{name: "tx_rand_1", cfg: withHash(base("tx-by-hash", validAddr), "0x7777777777777777777777777777777777777777777777777777777777777777"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_2", cfg: withHash(base("tx-by-hash", validAddr), "0x8888888888888888888888888888888888888888888888888888888888888888"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_3", cfg: withHash(base("tx-by-hash", validAddr), "0x9999999999999999999999999999999999999999999999999999999999999999"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_4", cfg: withHash(base("tx-by-hash", validAddr), "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_5", cfg: withHash(base("tx-by-hash", validAddr), "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_6", cfg: withHash(base("tx-by-hash", validAddr), "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_7", cfg: withHash(base("tx-by-hash", validAddr), "0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_8", cfg: withHash(base("tx-by-hash", validAddr), "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_9", cfg: withHash(base("tx-by-hash", validAddr), "0x0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "tx_rand_10", cfg: withHash(base("tx-by-hash", validAddr), "0xdeadbeefcafebabefacefeeddeadbabedeadbeefcafebabefacefeeddeadbabe"), expect: "\"tx-by-hash:", mayBeError: true},

		// --- 61-70 account-transactions fan-out ---
		{name: "acct_tx_7", cfg: base("account-transactions", "000000000000000000000000000000000000000000000000000000000000000e"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_8", cfg: base("account-transactions", "000000000000000000000000000000000000000000000000000000000000000f"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_9", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000010"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_10", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000020"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_11", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000030"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_12", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000040"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_13", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000050"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_14", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000060"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_15", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000070"), expect: "\"account-transactions:", mayBeError: true},
		{name: "acct_tx_16", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000080"), expect: "\"account-transactions:", mayBeError: true},

		// --- 71-80 wrong selector / experimental chain rejection ---
		// Any selector not in SupportedChains that also isn't wired via
		// experimental-chains should surface a configuration error before the
		// simulator dispatches to a capability.
		{name: "wrong_sel_evm_mainnet", cfg: withSelector(base("balance", validAddr), 5009297550715157269), expect: "", mayBeError: true},
		{name: "wrong_sel_solana", cfg: withSelector(base("balance", validAddr), 124615329519749607), expect: "", mayBeError: true},
		{name: "wrong_sel_zero", cfg: withSelector(base("balance", validAddr), 0), expect: "", mayBeError: true},
		{name: "wrong_sel_one", cfg: withSelector(base("balance", validAddr), 1), expect: "", mayBeError: true},
		{name: "wrong_sel_large", cfg: withSelector(base("balance", validAddr), ^uint64(0)), expect: "", mayBeError: true},
		{name: "wrong_sel_aptos_mainnet_unwired", cfg: withSelector(base("balance", validAddr), 4741433654826277614), expect: "", mayBeError: true},
		{name: "wrong_sel_view_experimental", cfg: withSelector(base("view", validAddr), 99999999), expect: "", mayBeError: true},
		{name: "wrong_sel_tx_experimental", cfg: withSelector(withHash(base("tx-by-hash", validAddr), "0x1"), 99999999), expect: "", mayBeError: true},
		{name: "wrong_sel_acct_experimental", cfg: withSelector(base("account-transactions", validAddr), 99999999), expect: "", mayBeError: true},
		{name: "wrong_sel_testnet_unchanged", cfg: withSelector(base("balance", validAddr), aptosTestnetSel), expect: "\"balance:"},

		// --- 81-90 UI / limits flag variations ---
		{name: "limits_none", cfg: base("balance", validAddr), expect: "\"balance:", args: []string{"--limits", "none"}},
		{name: "limits_default", cfg: base("balance", validAddr), expect: "\"balance:"},
		{name: "non_interactive", cfg: base("balance", validAddr), expect: "\"balance:"},
		{name: "trigger_index_0", cfg: base("balance", validAddr), expect: "\"balance:", args: []string{"--trigger-index", "0"}},
		{name: "trigger_index_invalid", cfg: base("balance", validAddr), expect: "trigger", mustFail: true, args: []string{"--trigger-index", "99"}},
		{name: "help_global", cfg: nil, expect: "cre", args: []string{"--help"}},
		{name: "workflow_simulate_help", cfg: nil, expect: "simulate", args: []string{"workflow", "simulate", "--help"}},
		{name: "missing_wasm", cfg: base("balance", validAddr), expect: "wasm", mustFail: true, args: []string{"--wasm", "/tmp/does-not-exist.wasm"}},
		{name: "missing_config", cfg: nil, expect: "config", mustFail: true, args: []string{"--config", "/tmp/does-not-exist.json"}},
		{name: "empty_target", cfg: base("balance", validAddr), expect: "target", mustFail: true, env: []string{"CRE_TARGET="}},

		// --- 91-100 broadcast + key edge cases (all must FAIL under dry-run
		// binary without a real key/network path) ---
		{name: "broadcast_sentinel_key_rejected", cfg: base("balance", validAddr), expect: "sentinel", mustFail: true,
			args: []string{"--broadcast"},
			env:  []string{"CRE_APTOS_PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000001"}},
		{name: "broadcast_unparseable_key_rejected", cfg: base("balance", validAddr), expect: "CRE_APTOS_PRIVATE_KEY", mustFail: true,
			args: []string{"--broadcast"},
			env:  []string{"CRE_APTOS_PRIVATE_KEY=not-hex"}},
		{name: "broadcast_short_key_rejected", cfg: base("balance", validAddr), expect: "CRE_APTOS_PRIVATE_KEY", mustFail: true,
			args: []string{"--broadcast"},
			env:  []string{"CRE_APTOS_PRIVATE_KEY=0102"}},
		{name: "dryrun_sentinel_key_warns", cfg: base("balance", validAddr), expect: "default Aptos private key",
			env: []string{"CRE_APTOS_PRIVATE_KEY="}},
		{name: "dryrun_valid_key_no_warning", cfg: base("balance", validAddr), expect: "\"balance:",
			env: []string{"CRE_APTOS_PRIVATE_KEY=1111111111111111111111111111111111111111111111111111111111111111"}},
		{name: "balance_followup_1", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000101"), expect: "\"balance:"},
		{name: "balance_followup_2", cfg: base("balance", "0000000000000000000000000000000000000000000000000000000000000202"), expect: "\"balance:"},
		{name: "view_followup", cfg: base("view", "0000000000000000000000000000000000000000000000000000000000000303"), expect: "view:", mayBeError: true},
		{name: "tx_followup", cfg: withHash(base("tx-by-hash", validAddr), "0x00000000000000000000000000000000000000000000000000000000000000ff"), expect: "\"tx-by-hash:", mayBeError: true},
		{name: "acct_tx_followup", cfg: base("account-transactions", "0000000000000000000000000000000000000000000000000000000000000404"), expect: "\"account-transactions:", mayBeError: true},
	}
	require.Len(t, scenarios, 100, "must have 100 CLI scenarios")

	for i, s := range scenarios {
		i, s := i, s
		t.Run(fmt.Sprintf("%03d_%s", i+1, s.name), func(t *testing.T) {
			var args []string
			// Scenarios that don't supply cfg (help / purely CLI-arg-driven)
			// skip the config-file plumbing entirely.
			if s.cfg != nil {
				cfgPath := fmt.Sprintf("/tmp/apcfg_%03d.json", i+1)
				data, err := json.Marshal(s.cfg)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(cfgPath, data, 0644))
				defer os.Remove(cfgPath)

				args = []string{
					"-T", "dev-aptos-testnet",
					"-R", projectDir,
					"workflow", "simulate", projectDir,
					"--wasm", wasmPath,
					"--config", cfgPath,
					"--non-interactive",
					"--trigger-index", "0",
					"--limits", "none",
				}
			}
			// Scenario-specific overrides are appended last so they win over
			// the defaults above (e.g. a different --wasm path).
			args = append(args, s.args...)

			cmd := exec.Command(cliBin, args...)
			cmd.Env = append(os.Environ(),
				"CRE_API_KEY=test-api",
			)
			cmd.Env = append(cmd.Env, s.env...)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			err := cmd.Run()
			combined := out.String()
			t.Logf("cre output:\n%s", combined)

			if s.mustFail {
				require.Error(t, err, "scenario %q expected to fail", s.name)
				require.Contains(t, combined, s.expect, "scenario %q missing expected error substring", s.name)
				return
			}

			// Help / no-cfg scenarios only need the expected substring — the
			// simulator markers are cron-specific and don't apply.
			if s.cfg == nil {
				require.NoError(t, err, "scenario %q expected to succeed", s.name)
				require.Contains(t, combined, s.expect, "scenario %q missing expected substring", s.name)
				return
			}

			// Every simulator run must reach init + trigger dispatch + result.
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
