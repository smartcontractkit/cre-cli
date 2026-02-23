# Test Scenario Classification Matrix

> Classifies every test scenario from `.qa-developer-runbook.md` into one of three automation tiers, with rationale for each classification.

---

## Classification Key

| Tier | Label | Meaning | Execution |
|------|-------|---------|-----------|
| **S** | Script-Automated | Deterministic, repeatable, no interpretation needed | CI pipeline, Go test, shell script |
| **AI** | AI-Augmented | Requires output interpretation, adaptive flow, or interactive handling | Claude Code agent, PTY wrapper |
| **M** | Manual | Requires human visual judgment, physical browser, or subjective assessment | Human tester |

### Decision Criteria

A test is classified as **Script (S)** when:
- The expected output is deterministic and can be validated with exact string match, exit code, or file existence
- No interactive input is required (or `--non-interactive` / flags can substitute)
- The test does not depend on external service state that changes unpredictably

A test is classified as **AI (AI)** when:
- The output requires semantic interpretation (e.g., "is this simulation result reasonable?")
- Interactive terminal input is needed (arrow keys, prompts, Esc)
- The test needs to adapt based on prior results (e.g., skip deploy if init failed)
- Error diagnosis requires reading and understanding compiler/runtime output
- The test involves driving a flow against real external services with variable responses

A test is classified as **Manual (M)** when:
- Visual rendering must be evaluated by a human eye (colors, layout, alignment)
- A physical browser is needed (OAuth redirect, SSO)
- The assessment is subjective ("does this feel right?")

---

## Section 2: Build and Smoke Test

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 2.1 | `make build` succeeds | **S** | Exit code check; deterministic |
| 2.2.1 | `./cre --help` shows grouped commands | **S** | String match on expected command groups |
| 2.2.2 | `./cre version` prints version string | **S** | Regex match on version format |
| 2.2.3 | `./cre init --help` shows init flags | **S** | String match for `-p`, `-t`, `-w`, `--rpc-url` |
| 2.2.4 | `./cre workflow --help` shows subcommands | **S** | String match for deploy, simulate, activate, pause, delete |
| 2.2.5 | `./cre secrets --help` shows subcommands | **S** | String match for create, update, delete, list, execute |
| 2.2.6 | `./cre account --help` shows subcommands | **S** | String match for link-key, unlink-key, list-key |
| 2.2.7 | `./cre login --help` shows login description | **S** | String match |
| 2.2.8 | `./cre whoami --help` shows whoami description | **S** | String match |
| 2.2.9 | `./cre nonexistent` shows unknown command error | **S** | Exit code non-zero + "unknown command" in stderr |
| -- | All commands in help match docs/ | **S** | Parse `--help` output, compare against `docs/cre_*.md` file list |
| -- | No panics on any `--help` call | **S** | Run all `--help` variants, check exit code 0 and no "panic" in output |
| -- | Global flags (`-v`, `-e`, `-R`, `-T`) appear on all commands | **S** | Parse each command's `--help`, check for global flags |

**Summary**: 100% script-automatable. All checks are deterministic string/exit-code assertions.

---

## Section 3: Unit and E2E Test Suite

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 3.1 | `make lint` passes | **S** | Exit code check |
| 3.2 | `make test` passes | **S** | Exit code check |
| 3.3 | `make test-e2e` passes | **S** | Exit code check; already in CI |

**Summary**: 100% script-automatable. Already runs in CI.

---

## Section 4: Account Creation and Authentication

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 4.1 | Create CRE account at cre.chain.link | **M** | Requires browser signup, email verification, human CAPTCHA |
| 4.2 | `cre login` -- browser OAuth flow | **M** | Opens browser, requires human to complete OAuth redirect |
| 4.3 | `cre whoami` -- displays account info | **S** | String match on email/org ID; works with `CRE_API_KEY` |
| 4.4 | `cre logout` -- clears credentials | **S** | Check exit code, verify `~/.cre/cre.yaml` deleted |
| 4.5 | Auto-login prompt on auth-required command | **AI** | CLI shows TTY prompt "Would you like to log in?"; need PTY to detect |
| 4.6 | API key auth via `CRE_API_KEY` | **S** | Set env var, run `cre whoami`, check output |

**Summary**: 2 manual (browser-dependent), 3 script, 1 AI (TTY prompt detection).

### Rationale Details

**4.1 (Manual)**: Account creation at cre.chain.link involves a web form, email verification, and potentially CAPTCHA. No API exists for this. This is a one-time setup, not a per-release test.

**4.2 (Manual)**: The `cre login` command opens a browser and requires the user to complete an OAuth2 PKCE flow at Auth0. The callback server on `localhost:53682` receives the token, but the human must interact with the browser. An AI agent could potentially drive this with browser automation (Playwright/Puppeteer), but the Auth0 login page has anti-bot protections that make this fragile. Recommendation: use `CRE_API_KEY` for automated testing; test login flow manually.

**4.5 (AI)**: When a user runs an auth-gated command without being logged in, the CLI displays a Bubbletea TUI prompt: "Would you like to log in?". This requires a PTY to detect and interact with. A script could use `expect`, but the rendering is terminal-dependent and the prompt text may change. An AI agent can read the PTY output and respond appropriately regardless of exact formatting.

---

## Section 5: Project Initialization (`cre init`)

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 5.1 | Interactive wizard -- full flow | **AI** | Requires PTY: arrow keys, Enter, text input across 5 steps |
| 5.2 | Non-interactive (all flags) -- Go | **S** | `cre init -p X -t 2 -w Y` + check file existence |
| 5.2 | Non-interactive (all flags) -- TS | **S** | `cre init -p X -t 3 -w Y` + check file existence |
| 5.3 | PoR template with RPC URL -- Go | **S** | `cre init -p X -t 1 -w Y --rpc-url Z` + check project.yaml contains URL |
| 5.3 | PoR template with RPC URL -- TS | **S** | `cre init -p X -t 4 -w Y --rpc-url Z` + check project.yaml contains URL |
| 5.4 | Init inside existing project | **S** | Run init twice; check new workflow dir created, project.yaml unchanged |
| 5.5 | Wizard cancel (Esc) | **AI** | Requires PTY: launch wizard, send Esc, verify clean exit |
| 5.6 | Directory already exists -- overwrite prompt | **AI** | Requires PTY: create dir, run init, interact with Yes/No prompt |

**Summary**: 4 script (non-interactive flags cover the critical path), 3 AI (interactive wizard/prompts), 0 manual.

### Rationale Details

**5.1 (AI)**: The interactive wizard uses Charm Bubbletea components. It renders a multi-step form with arrow-key selection (language, template), text input (project name, workflow name, RPC URL), and styled output (logo, progress, success box). A basic `expect` script would be brittle because:
- The rendered output includes ANSI escape codes for colors and cursor positioning
- Arrow key navigation requires specific escape sequences
- The wizard advances through steps with different layouts
- Error states (invalid input) change the rendering

An AI agent can:
- Read the PTY output and understand which step is active
- Send appropriate keystrokes
- Verify the wizard advances correctly even if formatting changes
- Handle error states gracefully

**5.6 (AI)**: The overwrite confirmation is a Bubbletea Yes/No prompt. While `expect` could handle this, the AI provides value by: understanding the prompt semantics, testing both Yes and No paths, and verifying the side effects (directory removed vs. abort).

---

## Section 6: Template Validation -- Go Templates

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 6.1a | Go HelloWorld init | **S** | `cre init -t 2` + file existence checks |
| 6.1b | Go HelloWorld build (`go build ./...`) | **S** | Exit code check |
| 6.1c | Go HelloWorld simulate | **S** | `cre workflow simulate --non-interactive --trigger-index=0` + exit code |
| 6.1d | Go HelloWorld simulate output correctness | **AI** | Interpret JSON result: is `{"Result": "Fired at ..."}` semantically correct? |
| 6.2a | Go PoR init | **S** | `cre init -t 1` + file existence checks |
| 6.2b | Go PoR build | **S** | Exit code check |
| 6.2c | Go PoR simulate | **S** | Exit code + "Write report succeeded" in output |
| 6.2d | Go PoR simulate output correctness | **AI** | Interpret output: is the PoR value reasonable? Is it a valid number? |

**Summary**: 6 script (init + build + simulate exit codes), 2 AI (semantic output validation).

### Rationale Details

**6.1d / 6.2d (AI)**: A script can check that the simulation exits successfully and output contains certain strings. But determining whether the simulation result is *correct* requires interpretation:
- For HelloWorld: the result should contain a timestamp near the current time
- For PoR: the result should be a plausible financial figure (not zero, not negative, not NaN)
- If the output format changes slightly (e.g., key names, number formatting), a script with exact match breaks; an AI adapts

The script tier handles the pass/fail gate (exit code 0 = simulation ran). The AI tier adds confidence that the output is semantically valid.

---

## Section 7: Template Validation -- TypeScript Templates

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 7.1a | TS HelloWorld init | **S** | `cre init -t 3` + file existence checks |
| 7.1b | TS HelloWorld install (`bun install`) | **S** | Exit code check |
| 7.1c | TS HelloWorld simulate | **S** | Exit code + "Hello world!" in output |
| 7.2a | TS PoR init | **S** | `cre init -t 4` + file existence checks |
| 7.2b | TS PoR install | **S** | Exit code check |
| 7.2c | TS PoR simulate | **S** | Exit code + output contains numeric value |
| 7.2d | TS PoR simulate output correctness | **AI** | Interpret output: is the PoR value a plausible number? |

**Summary**: 6 script, 1 AI (semantic validation of PoR output).

---

## Section 8: Workflow Simulate

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 8.1 | Basic simulate (Go + TS) | **S** | Exit code; already covered in Sections 6-7 |
| 8.2a | `--non-interactive --trigger-index 0` | **S** | Exit code; no prompts |
| 8.2b | `-g` (engine logs) | **S** | Check output contains engine log lines |
| 8.2c | `-v` (verbose) | **S** | Check output contains debug/verbose markers |
| 8.3 | HTTP payload (inline JSON) | **S** | Run with `--http-payload '{"key":"value"}'`, check exit code |
| 8.4 | EVM trigger flags | **S** | Run with `--evm-tx-hash` and `--evm-event-index`, check exit code |
| 8.5a | Missing workflow dir | **S** | Non-zero exit + "does not exist" in stderr |
| 8.5b | Non-interactive without trigger-index | **S** | Non-zero exit + "requires --trigger-index" |
| 8.5c | Bad trigger index (99) | **S** | Non-zero exit + "Invalid --trigger-index" |

**Summary**: 100% script-automatable. All simulate tests can be validated with exit codes and string matching.

---

## Section 9: Workflow Deploy / Pause / Activate / Delete

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 9.1 | Deploy | **AI** | Requires real auth + on-chain TX; AI interprets TX hash, verifies Etherscan link |
| 9.2a | `--yes` (skip confirm) | **S** with mocks; **AI** with real services |
| 9.2b | `-o` (custom output path) | **S** | Check file written to specified path |
| 9.2c | `--unsigned` (raw TX) | **S** with mocks; **AI** with real services |
| 9.3 | Pause | **AI** | Requires on-chain TX; verify state change |
| 9.4 | Activate | **AI** | Requires on-chain TX; verify state change |
| 9.5 | Delete | **AI** | Requires on-chain TX; verify removal |
| 9.6 | Full lifecycle (deploy -> pause -> activate -> delete) | **AI** | Sequential dependent steps; AI continues even if one fails |

**Summary**: With mocked services, most are script (existing E2E pattern). With real services, AI is needed for interpretation and adaptive execution.

### Rationale Details

**9.1-9.6 (AI with real services)**: The deploy lifecycle involves:
1. Real Ethereum transactions with gas estimation and confirmation
2. Variable response times (seconds to minutes for confirmation)
3. Transaction hashes that must be verified on Etherscan
4. Workflow IDs returned from the registry
5. State transitions that depend on prior operations

A script can execute these sequentially and check for "deployed successfully" strings. But an AI agent adds:
- Waiting intelligently for transaction confirmation (not fixed sleep)
- Verifying the Etherscan link actually works
- Understanding error messages when gas is insufficient or nonce is wrong
- Deciding whether to retry on transient network errors
- Continuing the lifecycle even if one step partially fails

For CI with mocked services, the existing E2E pattern (Tier 1 script) is sufficient.

---

## Section 10: Account Key Management

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 10.1 | `cre account link-key` | **AI** | TTY prompt for label input; on-chain TX |
| 10.2 | `cre account list-key` | **S** | Deterministic output format |
| 10.3 | `cre account unlink-key` | **AI** | TTY prompt for key selection; on-chain TX |

**Summary**: 1 script, 2 AI (TTY prompts + on-chain transactions).

---

## Section 11: Secrets Management

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 11.1 | Prepare secrets YAML file | **S** | File creation |
| 11.2 | `cre secrets create` | **S** with mocks; **AI** with real services |
| 11.3 | `cre secrets list` | **S** with mocks; **AI** with real services |
| 11.4 | `cre secrets update` | **S** with mocks; **AI** with real services |
| 11.5 | `cre secrets delete` | **S** with mocks; **AI** with real services |
| 11.6a | `--timeout 72h` (valid) | **S** | Exit code check |
| 11.6b | `--timeout 999h` (invalid) | **S** | Non-zero exit + error message |

**Summary**: With mocks, all are script. With real vault gateway, AI handles variable response interpretation.

---

## Section 12: Utility Commands

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 12.1 | `cre version` | **S** | String match |
| 12.2 | `cre update` | **AI** | Checks GitHub releases; behavior varies by current version and platform |
| 12.3 | `cre generate-bindings evm` | **S** | Exit code + generated files exist |
| 12.4 | Shell completion (bash/zsh/fish) | **S** | Pipe to `/dev/null`, check exit code |

**Summary**: 3 script, 1 AI (`cre update` has variable behavior).

### Rationale Details

**12.2 (AI)**: The `cre update` command:
- Checks GitHub releases API for newer versions
- Compares current version to latest release
- May or may not find an update
- On Windows, cannot self-replace the binary (known issue)
- Preview builds have version format mismatch with release tags

An AI agent can interpret the output regardless of whether an update is available, verify the download succeeds, and handle platform-specific edge cases.

---

## Section 13: Environment Switching

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 13.1 | Production (default) | **S** | Run `cre whoami` with API key, check success |
| 13.2 | Staging (`CRE_CLI_ENV=STAGING`) | **S** | Set env, run command, check URL or error |
| 13.3 | Development (`CRE_CLI_ENV=DEVELOPMENT`) | **S** | Set env, run command, check URL or error |
| 13.4 | Individual env var overrides | **S** | Set override, run with `-v`, check verbose output for overridden value |

**Summary**: 100% script-automatable.

---

## Section 14: Edge Cases and Negative Tests

### 14.1 Invalid Inputs

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 1 | `cre init -p "my project!"` | **S** | Non-zero exit + "invalid" in stderr |
| 2 | `cre init -p ""` | **S** | Check uses default name `my-project` |
| 3 | `cre init -w "my workflow"` | **S** | Non-zero exit + "invalid" in stderr |
| 4 | `cre init -t 999` | **S** | Non-zero exit + "not found" in stderr |
| 5 | `cre init --rpc-url ftp://bad` | **S** | SHOULD fail; currently passes (known bug) |
| 6 | `cre workflow simulate` (no path) | **S** | Non-zero exit + "accepts 1 arg(s)" |
| 7 | `cre workflow deploy` (no path) | **S** | Non-zero exit + "accepts 1 arg(s)" |
| 8 | `cre secrets create nonexistent.yaml` | **S** | Non-zero exit + "file not found" |

### 14.2 Auth Edge Cases

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 1 | `cre whoami` when logged out | **AI** | Shows login prompt (TTY) |
| 2 | `cre login` when already logged in | **M** | Requires browser |
| 3 | `cre logout` when already logged out | **S** | Graceful message, exit 0 |
| 4 | Corrupt `~/.cre/cre.yaml` then `cre whoami` | **AI** | Need to create corrupt file, interpret error, verify recovery prompt |

### 14.3 Network Edge Cases

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 1 | Deploy with insufficient ETH | **AI** | Requires real testnet + interpretation of TX failure |
| 2 | Deploy with invalid private key | **S** | Exit code + "invalid" in stderr |
| 3 | Simulate without Anvil installed | **S** | Only for EVM-trigger workflows; cron works without Anvil |
| 4 | Deploy when registry unreachable | **AI** | Requires real network; must interpret timeout/connection error |

### 14.4 Project Structure Edge Cases

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 1 | `cre init` in read-only directory | **S** | Permission error; exit code + message |
| 2 | Simulate with missing `workflow.yaml` | **S** | Exit code + "missing config" |
| 3 | Simulate with malformed `workflow.yaml` | **S** | Exit code + "parse error" |
| 4 | Ctrl+C mid-wizard | **AI** | Requires PTY: launch wizard, send SIGINT, verify clean exit + no partial files |

**Summary**: Mostly script (19 of 24). AI needed for TTY prompts, corrupt file recovery, and real-network error interpretation.

---

## Section 15: Wizard UX Verification

| # | Test | Tier | Rationale |
|---|------|------|-----------|
| 1 | Arrow Up/Down on language select | **AI** | PTY: send escape sequences, read rendered selection |
| 2 | Arrow Up/Down on template select | **AI** | PTY: same |
| 3 | Enter on selected item | **AI** | PTY: verify step advances |
| 4 | Esc at any step | **AI** | PTY: verify clean cancellation |
| 5 | Ctrl+C at any step | **AI** | PTY: verify clean cancellation |
| 6 | Invalid project name error feedback | **AI** | PTY: type invalid name, verify error shown inline |
| 7 | Invalid workflow name error feedback | **AI** | PTY: type invalid name, verify error shown |
| 8 | Default values (empty Enter) | **AI** | PTY: press Enter on empty field, verify default used |
| 9 | CRE logo renders correctly | **M** | Requires visual inspection -- ANSI art rendering varies by terminal |
| 10 | Colors visible on dark background | **M** | Subjective visual check |
| 11 | Selected items highlighted in blue | **M** | Subjective visual check |
| 12 | Error messages in orange | **M** | Subjective visual check |
| 13 | Help text at bottom of wizard | **AI** | PTY: check last lines contain help text |
| 14 | Completed steps shown as dim summary | **M** | Requires visual inspection of ANSI dim attribute |

**Summary**: 9 AI (PTY interaction), 4 manual (visual rendering), 1 that could go either way.

---

## Aggregate Classification

| Section | Script (S) | AI | Manual (M) | Total |
|---------|-----------|-----|------------|-------|
| 2. Build and Smoke | 13 | 0 | 0 | 13 |
| 3. Unit/E2E Suite | 3 | 0 | 0 | 3 |
| 4. Authentication | 3 | 1 | 2 | 6 |
| 5. Init | 4 | 3 | 0 | 7 |
| 6. Go Templates | 6 | 2 | 0 | 8 |
| 7. TS Templates | 6 | 1 | 0 | 7 |
| 8. Simulate | 9 | 0 | 0 | 9 |
| 9. Deploy Lifecycle | 2 | 6 | 0 | 8 |
| 10. Account Mgmt | 1 | 2 | 0 | 3 |
| 11. Secrets | 7 | 0 | 0 | 7 |
| 12. Utilities | 3 | 1 | 0 | 4 |
| 13. Environments | 4 | 0 | 0 | 4 |
| 14. Edge Cases | 19 | 4 | 1 | 24 |
| 15. Wizard UX | 0 | 9 | 5 | 14 |
| **TOTAL** | **80** | **29** | **8** | **117** |

### Percentage Breakdown

- **Script-automatable**: 80 / 117 = **68%** -- These should run in CI on every PR
- **AI-augmented**: 29 / 117 = **25%** -- These run pre-release with AI agent
- **Manual-only**: 8 / 117 = **7%** -- These require human judgment

### Coverage Improvement Over Current State

| Metric | Current | With Framework |
|--------|---------|----------------|
| Tests in CI | ~45 (unit + partial E2E) | 80 (script tier) |
| Tests automated (any form) | ~45 | 109 (script + AI) |
| Tests requiring human | ~103 (full runbook) | 8 |
| Templates tested | 3 of 5 | 5 of 5 |
| Platforms tested | 2 (Ubuntu, Windows) | 3 (+ macOS) |

---

## AI Agent Capability Requirements

Based on the 29 AI-classified tests, the AI agent needs these capabilities:

| Capability | Tests That Need It | Complexity |
|-----------|-------------------|------------|
| PTY/terminal interaction | 5.1, 5.5, 5.6, 10.1, 10.3, 14.2.1, 14.2.4, 14.4.4, 15.1-8, 15.13 | High -- requires PTY wrapper |
| Semantic output interpretation | 6.1d, 6.2d, 7.2d, 12.2 | Medium -- JSON parsing + heuristics |
| Real-service interaction | 9.1-9.6, 14.3.1, 14.3.4 | Medium -- needs credentials and network |
| Error diagnosis | 14.2.4 | Medium -- read error, suggest fix |
| Adaptive test flow | 9.6 | Low -- standard conditional logic |
| Report generation | All | Low -- template-based |

The most complex requirement is PTY interaction for the Bubbletea wizard. This is where AI provides the most value over traditional scripting, because:

1. The TUI rendering is non-trivial to parse programmatically (ANSI escape codes, cursor positioning, color sequences)
2. The exact output changes with terminal size, Bubbletea version, and content
3. An AI agent can "read" the rendered screen the way a human would, without needing exact byte-level parsing
4. When the wizard layout changes, the AI adapts without requiring test maintenance
