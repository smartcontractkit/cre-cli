# AI-Augmented Testing Framework Architecture

> Design document for a testing framework that combines deterministic scripts, AI-driven validation, and manual checks to catch cross-component breakage in the CRE CLI.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [Current State Analysis](#3-current-state-analysis)
4. [Framework Architecture](#4-framework-architecture)
5. [Test Layer Definitions](#5-test-layer-definitions)
6. [AI Agent Design](#6-ai-agent-design)
7. [Component Interaction Model](#7-component-interaction-model)
8. [Failure Detection Matrix](#8-failure-detection-matrix)
9. [Environment Requirements](#9-environment-requirements)
10. [Risk Analysis](#10-risk-analysis)

---

## 1. Executive Summary

The CRE CLI currently ships embedded templates that are the primary entry point for CRE developers, and a branch-gated dynamic template pull model is planned. Both source modes depend on Go and TypeScript SDKs, GraphQL APIs, on-chain contracts, and third-party packages -- all of which evolve independently. The current testing infrastructure validates these components in isolation using mocked services, which means cross-component breakage goes undetected until developers hit errors.

This document describes a three-tier testing framework:

- **Tier 1 -- Script-Automated Tests**: Deterministic, fast, CI-gated tests for template compilation, simulation, and CLI command correctness.
- **Tier 2 -- AI-Augmented Tests**: AI agents (Claude Code or equivalent) that perform exploratory validation, interpret ambiguous outputs, handle interactive flows, and generate structured test reports.
- **Tier 3 -- Manual Tests**: Human-only validation for visual UX, browser-based auth, and subjective quality assessment.

The goal is to shift the bulk of the current 103-test manual runbook (`.qa-developer-runbook.md`) into Tiers 1 and 2, leaving only ~10 tests that genuinely require human judgment.

---

## 2. Problem Statement

### 2.0 Template Source Assumptions

- Baseline runtime source: embedded templates (`cmd/creinit/template/workflow/**/*` via `go:embed`).
- Upcoming source (branch-gated): dynamic fetch from external template repository.
- Dynamic-mode validation is planned now and becomes required once branch-level interface details are available.

### 2.1 The Dependency Web

The developer experience depends on seven independently evolving components:

```
CLI Binary (Go, Cobra)
  |
  +-- Template Sources
  |     +-- Embedded templates (current baseline)
  |     +-- Dynamic pulled templates (upcoming, branch-gated)
  |     |
  |     +-- Go SDK: cre-sdk-go (pinned at v1.2.0 in go_module_init.go)
  |     |     +-- EVM Capabilities (v1.0.0-beta.5)
  |     |     +-- HTTP Capabilities (v1.0.0-beta.0)
  |     |     +-- Cron Capabilities (v1.0.0-beta.0)
  |     |
  |     +-- TS SDK: @chainlink/cre-sdk (^1.0.9 in package.json.tpl)
  |     |     +-- viem (2.34.0)
  |     |     +-- zod (3.25.76)
  |     |
  |     +-- Developer Toolchain (Go 1.25.5, Bun 1.2.21, Node 20.13.1, Anvil v1.1.0)
  |
  +-- GraphQL API (cre.chain.link/api)
  +-- Artifact Storage (presigned URL upload/download)
  +-- Workflow Registry (on-chain smart contract)
  +-- Vault DON (secrets gateway)
  +-- Auth0 (OAuth2 PKCE)
```

### 2.2 Why Things Break

| Scenario | What Happens | Current Detection |
|----------|-------------|-------------------|
| SDK team releases cre-sdk-go v1.3.0 with API changes | Go templates fail to compile after `cre init` | User reports |
| @chainlink/cre-sdk publishes v1.1.0 with breaking change | TS templates fail at `bun install` or runtime | User reports |
| GraphQL API adds required field | `cre workflow deploy` fails with cryptic error | User reports |
| Workflow Registry contract is upgraded | Deploy/pause/activate/delete fail | User reports |
| viem releases patch with behavior change | TS PoR template simulation produces wrong results | User reports (maybe never) |
| Bun version changes break TS bundling | `cre workflow simulate` fails for TS workflows | CI catches on matching version; user hits it on different version |

### 2.3 Scale of the Problem

- 5 templates today, growing to 10+ in the near term
- 2 languages (Go, TypeScript), potentially more
- 3 environments (DEV, STAGING, PRODUCTION)
- 3 platforms (macOS, Linux, Windows)
- Full matrix: 5 templates x 3 environments x 3 platforms = 45 test combinations, each requiring init + build + simulate

---

## 3. Current State Analysis

### 3.1 What Exists Today

**Unit Tests (ci-test-unit):**
- Cover individual packages: `cmd/creinit/`, `internal/validation/`, `internal/settings/`, etc.
- Run on every PR to `main` and `releases/**`
- Do NOT test templates end-to-end

**E2E Tests (ci-test-e2e):**
- Run on Ubuntu + Windows matrix
- Test 3 of 5 templates (Go PoR, Go HelloWorld, TS PoR)
- Mock all external services (GraphQL, Storage, Vault)
- Use pre-baked Anvil state for on-chain interactions
- Template 3 (TS HelloWorld) and Template 5 (TS ConfHTTP) are never tested

**System Tests (ci-test-system):**
- Disabled (`if: false` in CI)
- Would test against real Chainlink infrastructure if enabled

**Manual QA (`.qa-developer-runbook.md`):**
- 103 test cases across 16 sections
- Covers the full user journey: install -> login -> init -> simulate -> deploy -> manage
- Takes 2-4 hours per run
- Last report (Windows): 75 PASS, 1 FAIL, 18 SKIP, 9 N/A

### 3.2 What Is Missing

| Gap | Impact |
|-----|--------|
| Templates 3 and 5 have zero E2E coverage | Breakage goes completely undetected |
| No tests run against real external services | API contract changes slip through |
| No SDK version compatibility matrix | SDK updates break templates silently |
| No macOS CI testing | Platform-specific bugs missed |
| No interactive flow testing | Wizard/TUI bugs only found manually |
| No automated cross-component integration | Deploy pipeline changes break CLI workflows |
| System tests disabled | Full stack is never validated in CI |
| No dynamic-template fetch failure coverage | Remote source outages/ref drift can break init flows silently once enabled |

### 3.3 Test Infrastructure Details

The existing E2E tests follow this pattern:

1. **Binary build**: `TestMain` in `test/main_test.go` compiles the CLI binary to `$TMPDIR/cre`
2. **Anvil startup**: `StartAnvil()` in `test/common.go` launches a local Ethereum node with pre-baked state
3. **Mock servers**: `httptest.NewServer` serves fake GraphQL, Storage, and Vault responses
4. **Environment injection**: Override URLs via env vars (`CRE_CLI_GRAPHQL_URL`, `CRE_VAULT_DON_GATEWAY_URL`, etc.)
5. **CLI invocation**: `exec.Command(CLIPath, "workflow", "simulate", ...)` runs the binary as a subprocess
6. **Output assertion**: `require.Contains(t, output, "Workflow deployed successfully")`

Key limitation: the mock servers return hardcoded JSON. If the real API changes its response shape, field names, or error format, the mocks still return the old format and tests pass.

---

## 4. Framework Architecture

### 4.1 Three-Tier Testing Model

```
+------------------------------------------------------------------+
|                                                                    |
|  TIER 1: SCRIPT-AUTOMATED (Deterministic, CI-Gated)              |
|                                                                    |
|  +-------------------+  +-------------------+  +---------------+  |
|  | Template           |  | CLI Command       |  | SDK Version   |  |
|  | Compatibility     |  | Smoke Tests       |  | Matrix        |  |
|  | Tests             |  | (existing E2E)    |  | Tests         |  |
|  +-------------------+  +-------------------+  +---------------+  |
|                                                                    |
|  Runs: Every PR, every SDK release, nightly                       |
|  Time: 5-15 minutes                                               |
|  Gate: Blocks merge on failure                                    |
|                                                                    |
+------------------------------------------------------------------+
|                                                                    |
|  TIER 2: AI-AUGMENTED (Interpretive, Report-Generating)           |
|                                                                    |
|  +-------------------+  +-------------------+  +---------------+  |
|  | Full User Journey |  | Interactive Flow  |  | Error         |  |
|  | Validation        |  | Testing           |  | Diagnosis     |  |
|  | (init->deploy->   |  | (wizard, prompts, |  | (interpret    |  |
|  |  manage lifecycle) |  |  confirmation)    |  |  failures,    |  |
|  +-------------------+  +-------------------+  |  suggest fix)  |  |
|                                                 +---------------+  |
|  Runs: Pre-release, on-demand, after major changes                |
|  Time: 15-45 minutes                                              |
|  Gate: Generates report for human review                          |
|                                                                    |
+------------------------------------------------------------------+
|                                                                    |
|  TIER 3: MANUAL (Human Judgment Required)                         |
|                                                                    |
|  +-------------------+  +-------------------+  +---------------+  |
|  | Visual UX         |  | Browser Auth      |  | Cross-OS      |  |
|  | (colors, layout,  |  | Flow              |  | Visual Parity |  |
|  |  rendering)       |  | (OAuth redirect)  |  |               |  |
|  +-------------------+  +-------------------+  +---------------+  |
|                                                                    |
|  Runs: Pre-release only                                           |
|  Time: 30-60 minutes                                              |
|  Gate: Human sign-off                                             |
|                                                                    |
+------------------------------------------------------------------+
```

### 4.2 How the Tiers Interact

```
SDK Release / CLI PR / Nightly Schedule
        |
        v
  +-- Tier 1 (CI) --+
  |  All template    |
  |  compatibility   |---> FAIL? --> Block merge, notify
  |  tests           |
  +------------------+
        |
        | PASS
        v
  +-- Tier 2 (AI) --+
  |  Full journey    |
  |  validation      |---> Generates .qa-test-report-YYYY-MM-DD.md
  |  + interactive   |     for human review
  |  flows           |
  +------------------+
        |
        | Report reviewed
        v
  +-- Tier 3 (Human) +
  |  Visual + browser |
  |  only checks     |---> Final sign-off
  +-------------------+
```

---

## 5. Test Layer Definitions

### 5.1 Tier 1: Template Compatibility Tests

**Purpose**: Catch template breakage within minutes of a PR or SDK release.

**What it tests**:

For every template (IDs 1-5):
1. `cre init` with all required flags (non-interactive)
2. Dependency installation (`go build ./...` for Go, `bun install` for TS)
3. Compilation to WASM (`go build -o tmp.wasm` with `GOOS=wasip1 GOARCH=wasm` for Go, `bun run build` for TS)
4. Simulation (`cre workflow simulate <workflow> --non-interactive --trigger-index=0`)
5. For Go templates: `go test ./...` (workflow unit tests)

**What it does NOT test**:
- Real API interactions (still mocked)
- Deploy/pause/activate/delete (requires auth + on-chain TX)
- Interactive wizard flows
- Browser auth

**Implementation approach**: Extend existing E2E test pattern in `test/` to cover all 5 templates. This is pure Go test code with no AI involvement.

**Key file additions**:
- `test/template_compatibility_test.go` -- data-driven test that iterates over all template IDs
- Auto-discovery of templates from `languageTemplates` in `cmd/creinit/creinit.go` (or a shared registry)

### 5.2 Tier 1: SDK Version Matrix Tests

**Purpose**: Detect breakage when SDK versions change.

**What it tests**:

For each template x SDK version combination:
1. Init with current CLI binary
2. Override SDK version (modify `go.mod` or `package.json` after scaffolding)
3. Build + simulate

**Matrix dimensions**:
- Go SDK: current pinned version, latest release, latest pre-release
- TS SDK: current pinned version, latest npm release
- Third-party (viem, zod): current pinned, latest

**Trigger**: Scheduled (nightly) or on SDK release (via GitHub webhook or repository_dispatch).

### 5.3 Tier 2: AI-Driven Full Journey Tests

**Purpose**: Validate the complete developer experience from install through deploy lifecycle.

**What it tests**:

The AI agent executes the full journey from `.qa-developer-runbook.md`:
1. Build CLI from source (or use pre-built binary)
2. Smoke test all commands (`--help`, `version`, etc.)
3. Authenticate (API key for CI, browser for manual)
4. Init all templates (interactive and non-interactive)
5. Simulate all templates
6. Deploy -> Pause -> Activate -> Delete lifecycle
7. Secrets CRUD
8. Account key management
9. Edge cases and negative tests
10. Environment switching

**What makes this AI-driven (not just scripted)**:
- **Output interpretation**: The AI reads simulation output and determines if the result is semantically correct (not just "contains string X")
- **Error diagnosis**: When a step fails, the AI analyzes the error, checks logs, and suggests root cause
- **Adaptive flow**: If one template fails to init, the AI still proceeds with other templates rather than aborting
- **Interactive handling**: The AI can drive TTY-based prompts using tools like `expect`, pseudo-TTY wrappers, or direct stdin writing
- **Report generation**: The AI produces a structured test report matching `.qa-test-report-template.md`

### 5.4 Tier 2: Interactive Flow Testing

**Purpose**: Validate the Charm Bubbletea wizard and other TTY-dependent flows.

**What it tests**:
- Init wizard step-by-step navigation
- Arrow key selection for language and template
- Input validation feedback (invalid project names, workflow names)
- Default value behavior (empty Enter)
- Esc/Ctrl+C cancellation
- Overwrite confirmation prompts

**AI approach**: The AI agent uses a PTY wrapper (e.g., `expect`-style tool, or node-pty/script) to:
1. Launch the CLI in a pseudo-terminal
2. Read rendered output
3. Send keystrokes (arrows, Enter, Esc)
4. Validate that the wizard advances correctly
5. Verify no garbled output or rendering artifacts

### 5.5 Tier 3: Manual-Only Tests

**Purpose**: Validate things that require human visual and cognitive judgment.

**What it tests**:
- CRE logo renders correctly (no garbled characters)
- Colors visible on dark/light terminal backgrounds
- Selected items clearly highlighted in blue
- Error messages visible in orange
- Help text visible at bottom of wizard
- Browser OAuth redirect works end-to-end
- Cross-terminal rendering (Terminal.app, iTerm2, VS Code, Windows Terminal)

---

## 6. AI Agent Design

### 6.1 Agent Architecture

```
+--------------------------------------------------+
|  AI Test Agent (Claude Code or equivalent)        |
|                                                    |
|  Inputs:                                          |
|    - .qa-developer-runbook.md (test spec)         |
|    - .qa-test-report-template.md (output format)  |
|    - CLI binary (pre-built or source)             |
|    - Environment config (API key, RPC URLs)       |
|    - Platform info (OS, terminal type)            |
|                                                    |
|  Capabilities:                                    |
|    - Shell command execution                      |
|    - File system read/write                       |
|    - PTY interaction (for wizard/prompts)         |
|    - HTTP requests (for API health checks)        |
|    - Structured output generation                 |
|                                                    |
|  Outputs:                                         |
|    - .qa-test-report-YYYY-MM-DD.md                |
|    - Exit code (0 = all pass, 1 = failures)       |
|    - Artifact directory (screenshots, logs)       |
|                                                    |
+--------------------------------------------------+
```

### 6.2 Agent Execution Model

The AI agent operates in a structured loop:

```
FOR each section in runbook:
  1. READ test specification
  2. DETERMINE if test is executable in current environment
     - Skip browser tests if no display
     - Skip TTY tests if no PTY available
     - Skip deploy tests if no credentials
  3. EXECUTE commands
  4. CAPTURE output (stdout, stderr, exit code)
  5. INTERPRET results:
     - Compare against expected behavior in runbook
     - Classify as PASS / FAIL / SKIP / BLOCKED
     - For FAIL: analyze error, check common causes
  6. RECORD in report template
  7. CONTINUE to next test (do not abort on failure)
```

### 6.3 What Makes AI Valuable vs. Plain Scripts

| Capability | Script | AI Agent |
|-----------|--------|----------|
| Run `cre init -t 2 -w test` and check exit code | Yes | Yes |
| Verify output contains "Project created successfully" | Yes | Yes |
| Determine if simulation output is semantically correct | No | Yes -- can interpret JSON result, check data types, validate business logic |
| Handle unexpected prompts or error messages | No -- aborts or hangs | Yes -- reads prompt, decides action |
| Navigate interactive wizard with arrow keys | Possible with expect scripts, but brittle | Yes -- reads rendered TUI, understands layout |
| Diagnose why a template fails to compile | No -- just reports exit code | Yes -- reads compiler errors, cross-references SDK docs |
| Adapt test order when dependencies fail | No -- follows fixed script | Yes -- skips downstream tests, notes in report |
| Generate human-readable test report | Template fill only | Yes -- writes analysis, recommendations, severity |
| Detect regressions in output format | Only with regex/exact match | Yes -- understands semantic changes |

### 6.4 AI Agent Limitations

| Limitation | Mitigation |
|-----------|------------|
| Non-deterministic output interpretation | Use Tier 1 scripts for critical pass/fail gates; AI for exploratory validation |
| Cost per run (~$5-15 for full journey) | Run Tier 2 only pre-release and on-demand, not on every PR |
| Latency (15-45 min for full journey) | Parallelize template tests; run Tier 1 first as fast gate |
| Cannot see pixels (visual UX) | Keep Tier 3 manual for visual verification |
| PTY interaction is complex | Provide structured PTY wrapper library; fall back to `--non-interactive` |
| May produce false positives | All AI reports reviewed by human before blocking release |

---

## 7. Component Interaction Model

### 7.1 Which Tests Validate Which Integration Points

```
Integration Point              | Tier 1 (Script) | Tier 2 (AI) | Tier 3 (Manual)
-------------------------------|------------------|-------------|----------------
CLI -> Embedded Templates      |       X          |      X      |
Templates -> Go SDK            |       X          |      X      |
Templates -> TS SDK            |       X          |      X      |
Templates -> Third-party deps  |       X          |      X      |
CLI -> WASM Compiler           |       X          |      X      |
CLI -> Simulation Engine       |       X          |      X      |
CLI -> GraphQL API             |                  |      X*     |
CLI -> Artifact Storage        |                  |      X*     |
CLI -> Workflow Registry       |                  |      X*     |
CLI -> Vault DON               |                  |      X*     |
CLI -> Auth0                   |                  |             |       X
CLI -> TUI (Bubbletea)         |                  |      X      |       X
CLI -> Terminal rendering      |                  |             |       X

* When credentials and environment access are available
```

### 7.2 Test Trigger Matrix

```
Event                          | Tier 1 Templates | Tier 1 SDK Matrix | Tier 2 AI Journey | Tier 3 Manual
-------------------------------|------------------|-------------------|-------------------|-------------
PR to main                     |       X          |                   |                   |
PR to releases/**              |       X          |                   |        X          |
Tag push (v*)                  |       X          |        X          |        X          |       X
SDK release (cre-sdk-go)       |       X          |        X          |                   |
SDK release (@chainlink/cre-sdk)|      X          |        X          |                   |
Nightly schedule               |       X          |        X          |        X          |
On-demand                      |       X          |        X          |        X          |       X
```

---

## 8. Failure Detection Matrix

This maps every known failure mode to the tier that would catch it:

| Failure Mode | Example | Caught By |
|-------------|---------|-----------|
| Template source incompatible with new SDK | `cre-sdk-go` renames `ExecutionResult` | Tier 1 (compile fails) |
| TS SDK minor version breaks template | `@chainlink/cre-sdk@1.1.0` changes API | Tier 1 SDK matrix (build fails) |
| Third-party dep breaking change | `viem@2.35.0` changes ABI encoding | Tier 1 SDK matrix (simulate fails) |
| Go toolchain change breaks WASM build | New Go version changes WASM output | Tier 1 (compile fails) |
| Simulation produces wrong result | PoR template returns `0` instead of price | Tier 2 (AI interprets output) |
| GraphQL API field renamed | `organizationId` -> `orgId` | Tier 2 (real API test fails) |
| Workflow Registry ABI change | `UpsertWorkflow` signature changes | Tier 2 (deploy fails with real contract) |
| Auth flow breaks | Auth0 callback URL changes | Tier 3 (browser test fails) |
| Wizard rendering broken | Bubbletea update garbles layout | Tier 2 (PTY test) + Tier 3 (visual) |
| Platform-specific path bug | Windows backslash in template path | Tier 1 (Windows CI matrix) |
| RPC URL validation missing | `ftp://bad` accepted (known bug) | Tier 1 (negative test) |
| Secrets YAML format mismatch | CLI expects different format than docs | Tier 2 (AI follows runbook, hits error) |
| Self-update broken on Windows | Binary replacement fails | Tier 2 (AI runs `cre update` on Windows) |
| Template missing from registry | New template added to code but not to tests | Tier 1 (auto-discovery from `languageTemplates`) |

---

## 9. Environment Requirements

### 9.1 Tier 1 (CI)

| Requirement | Purpose | Existing? |
|------------|---------|-----------|
| Go 1.25.5 | Build CLI, compile Go templates | Yes (CI) |
| Bun 1.2.21 | Install TS deps, bundle TS templates | Yes (CI) |
| Node.js 20.13.1 | TS runtime support | Yes (CI) |
| Foundry/Anvil v1.1.0 | Local Ethereum for simulation | Yes (CI) |
| Ubuntu runner | Primary CI platform | Yes |
| Windows runner | Cross-platform CI | Yes |
| macOS runner | Cross-platform CI | NO -- needs to be added |

### 9.2 Tier 2 (AI Agent)

| Requirement | Purpose | Notes |
|------------|---------|-------|
| All Tier 1 requirements | Same toolchain | |
| Claude Code CLI or API | AI agent runtime | Requires API key or CLI installation |
| CRE_API_KEY | Auth for API-dependent tests | Must be scoped to test environment |
| PTY support | Interactive wizard testing | `script` command or `node-pty` |
| Network access | Test against real APIs | STAGING environment recommended |
| ETH_PRIVATE_KEY (Sepolia) | On-chain operations | Testnet only; dedicated test wallet |

### 9.3 Tier 3 (Manual)

| Requirement | Purpose | Notes |
|------------|---------|-------|
| Physical machine or VM | Visual verification | Multiple OS |
| Browser | OAuth flow testing | Chrome, Firefox, Safari |
| Multiple terminals | Rendering comparison | Terminal.app, iTerm2, VS Code, Windows Terminal |
| Display | Visual inspection | Cannot be headless |

---

## 10. Risk Analysis

### 10.1 Risks of the Framework Itself

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| AI agent produces false confidence (PASS when it should be FAIL) | Medium | High | Tier 1 scripts are the hard gate; Tier 2 is advisory and reviewed by human |
| AI agent cost escalates as templates grow | Medium | Medium | Cap concurrent runs; use Tier 1 for fast feedback, Tier 2 only pre-release |
| PTY wrapper breaks across OS | High | Medium | Provide platform-specific wrappers; fall back to `--non-interactive` |
| Test environment credentials leak | Low | High | Use short-lived tokens; dedicated test org; rotate keys |
| Flaky tests due to network/RPC issues | Medium | Medium | Retry logic; health checks before test execution; mock fallback |
| Maintaining test infrastructure becomes its own burden | Medium | Medium | Data-driven tests that auto-discover templates; minimal custom per-template logic |

### 10.2 What This Framework Will NOT Catch

- Zero-day vulnerabilities in dependencies
- Bugs that only manifest at scale (100+ workflows)
- Performance regressions (requires benchmarking, not functional tests)
- Issues with specific user wallet configurations
- Bugs introduced by user modifications to scaffolded templates
