# CI/CD Integration Design for Automated Template Testing

> Design for integrating template compatibility tests, SDK version matrix validation, and AI-augmented testing into the CRE CLI's GitHub Actions CI/CD pipeline, including branch-gated dynamic template source validation.

---

## Table of Contents

1. [Current CI/CD Landscape](#1-current-cicd-landscape)
2. [Proposed Workflow Additions](#2-proposed-workflow-additions)
3. [Template Compatibility Job](#3-template-compatibility-job)
4. [SDK Version Matrix Job](#4-sdk-version-matrix-job)
5. [AI Agent Integration Job](#5-ai-agent-integration-job)
6. [Cross-Repository Triggers](#6-cross-repository-triggers)
7. [Environment and Secrets Management](#7-environment-and-secrets-management)
8. [Cost and Runtime Analysis](#8-cost-and-runtime-analysis)
9. [Rollout Plan](#9-rollout-plan)
10. [Monitoring and Alerting](#10-monitoring-and-alerting)

---

## 1. Current CI/CD Landscape

### 1.1 Existing Workflows

| Workflow | File | Trigger | Jobs |
|----------|------|---------|------|
| PR Checks | `pull-request-main.yml` | PR to `main`, `releases/**` | ci-lint, ci-lint-misc, ci-test-unit, ci-test-e2e (Ubuntu + Windows), ci-test-system (DISABLED), tidy |
| Preview Build | `preview-build.yml` | PR with `preview` label | Build Linux/Darwin/Windows binaries (unsigned) |
| Build & Release | `build-and-release.yml` | Tag push `v*` | Build + sign + notarize + GitHub Release (draft) |
| Doc Generation | `generate-docs.yml` | PR to `main`, `releases/**` | Regenerate docs, fail if changed |
| Upstream Check | `check-upstream-abigen.yml` | PR + manual | Check go-ethereum abigen fork for updates |

### 1.2 Existing Test Matrix

```
ci-test-unit:
  Runner: ubuntu-latest
  Command: go test -v $(go list ./... | grep -v -e usbwallet -e test)
  Coverage: All packages except usbwallet and test/

ci-test-e2e:
  Matrix: [ubuntu-latest, windows-latest]
  Tools: Foundry v1.1.0, Bun (latest)
  Command: go test -p 5 -v -timeout 30m ./test/
  Coverage: Templates 1, 2, 4 (not 3 or 5)
  Mocking: All external services mocked

ci-test-system:
  Status: DISABLED (if: false)
  Would have tested: Full CRE OCR3 PoR system test
```

### 1.3 What is NOT Covered

- Template 3 (TS HelloWorld) and Template 5 (TS ConfHTTP) have zero test coverage
- macOS is not tested
- No tests run against real external services
- No SDK version compatibility testing
- No automatic testing when upstream SDKs release new versions
- No AI-augmented testing in CI

---

## 2. Proposed Workflow Additions

### 2.0 Template Source Modes in CI

- Embedded template compatibility is the required baseline gate.
- Dynamic template pull compatibility is introduced as advisory first, then promoted to merge gate only after stability thresholds are met.
- Dynamic-mode jobs are branch-gated until upstream branch/repo integration is available.

### 2.1 New Jobs in Existing Workflows

```
pull-request-main.yml (MODIFIED):
  Existing jobs: [unchanged]
  New jobs:
    +-- ci-test-template-compat     # All 5 templates: init + build + simulate
    +-- ci-test-template-compat-mac # macOS template test (optional, per label)

New workflows:
    +-- sdk-version-matrix.yml       # Nightly + on SDK release
    +-- ai-validation.yml            # Pre-release + on-demand
```

### 2.2 Trigger Map

```
Event                              | template-compat | sdk-matrix | ai-validation
-----------------------------------|-----------------|------------|---------------
PR to main                         |       X         |            |
PR to releases/**                  |       X         |            |       X
Tag push v*                        |       X         |     X      |       X
Nightly schedule (cron)            |       X         |     X      |
On-demand (workflow_dispatch)      |       X         |     X      |       X
SDK release (repository_dispatch)  |       X         |     X      |

Additional planned trigger when dynamic mode is active:

- Template repo change event (cross-repo dispatch/webhook, with polling fallback) triggers dynamic compatibility validation.
```

### 2.3 Dependency Diagram

```
PR opened
    |
    v
+-- ci-lint ----+
+-- ci-lint-misc +-------> ci-test-unit
+-- tidy --------+              |
                                v
                    ci-test-e2e (existing)
                                |
                                v
                  ci-test-template-compat (NEW)
                                |
                                v
                       PR ready to merge
```

The template compatibility job runs after (or in parallel with) the existing E2E tests. It should NOT be a dependency of E2E tests -- they are independent validation layers.

### 2.4 Gate Policy Defaults

Default policy for this framework:

- **Required merge gates:** deterministic checks only (template compatibility, deterministic smoke/negative-path checks).
- **Advisory checks by default:** AI-driven and nightly exploratory coverage (`ai-validation`, expanded diagnostics).
- **Manual/browser checks:** non-gating and tracked as manual-signoff evidence.

This keeps merge decisions objective while preserving deeper diagnostic coverage outside the critical path.

### 2.5 Reporting Status and Reason Taxonomy

Use this status vocabulary across CI summaries and reports:

- `PASS`
- `FAIL`
- `SKIP`
- `BLOCKED`

Use these first-level reason codes for consistency:

| Status Class | Reason Code | Meaning |
|---|---|---|
| `BLOCKED` | `BLOCKED_ENV` | Missing toolchain/dependency/runner prerequisite |
| `BLOCKED` | `BLOCKED_AUTH` | Missing/invalid credentials or auth context |
| `FAIL` | `FAIL_COMPAT` | Template compatibility suite failure |
| `FAIL` | `FAIL_TUI` | PTY/interactive flow regression |
| `FAIL` | `FAIL_NEGATIVE_PATH` | Expected error-path contract not met |
| `FAIL` | `FAIL_CONTRACT` | Source-mode/policy contract violation |
| `SKIP` | `SKIP_MANUAL` | Intentionally human-only validation |
| `SKIP` | `SKIP_PLATFORM` | Platform-scoped skip with explicit rationale |

---

## 3. Template Compatibility Job

### 3.1 Job Specification

```yaml
# Addition to pull-request-main.yml

ci-test-template-compat:
  name: "Template Compatibility (${{ matrix.os }})"
  runs-on: ${{ matrix.os }}
  strategy:
    fail-fast: false
    matrix:
      os: [ubuntu-latest, windows-latest]
      # macOS is expensive; consider adding macos-latest behind a label gate
  needs: []  # runs independently, no dependency on other jobs

  steps:
    - uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version-file: '.tool-versions'

    - name: Setup Bun
      uses: oven-sh/setup-bun@v2
      with:
        bun-version-file: '.tool-versions'

    - name: Setup Node
      uses: actions/setup-node@v4
      with:
        node-version-file: '.tool-versions'

    - name: Install Foundry
      uses: foundry-rs/foundry-toolchain@v1
      with:
        version: v1.1.0

    - name: Run Template Compatibility Tests
      run: go test -v -timeout 20m -run TestTemplateCompatibility ./test/
      env:
        CRE_API_KEY: "test-api"
```

### 3.2 Test Implementation Structure

The test file `test/template_compatibility_test.go` follows the existing E2E pattern:

```
TestTemplateCompatibility (parent test)
  |
  +-- TestTemplateCompatibility/GoPoR_Template1
  |     1. cre init -p test-go-por -t 1 -w por-wf --rpc-url <default>
  |     2. Verify: go.mod, main.go, workflow.go, workflow_test.go, contracts/
  |     3. go build ./...
  |     4. cre workflow simulate por-wf --non-interactive --trigger-index=0
  |
  +-- TestTemplateCompatibility/GoHelloWorld_Template2
  |     1. cre init -p test-go-hello -t 2 -w hello-wf
  |     2. Verify: go.mod, main.go
  |     3. go build ./...
  |     4. cre workflow simulate hello-wf --non-interactive --trigger-index=0
  |
  +-- TestTemplateCompatibility/TSHelloWorld_Template3
  |     1. cre init -p test-ts-hello -t 3 -w hello-wf
  |     2. Verify: main.ts, package.json, tsconfig.json
  |     3. bun install
  |     4. cre workflow simulate hello-wf --non-interactive --trigger-index=0
  |
  +-- TestTemplateCompatibility/TSPoR_Template4
  |     1. cre init -p test-ts-por -t 4 -w por-wf --rpc-url <default>
  |     2. Verify: main.ts, package.json, contracts/abi/
  |     3. bun install
  |     4. cre workflow simulate por-wf --non-interactive --trigger-index=0
  |
  +-- TestTemplateCompatibility/TSConfHTTP_Template5
        1. cre init -p test-ts-conf -t 5 -w conf-wf
        2. Verify: main.ts, package.json
        3. bun install
        4. cre workflow simulate conf-wf --non-interactive --trigger-index=0
```

### 3.3 Mock Server Setup

Each sub-test sets up a mock GraphQL server (identical to existing E2E pattern):

```
Mock GraphQL server handles:
  POST /graphql:
    "getOrganization" -> {"data":{"getOrganization":{"organizationId":"test-org-id"}}}
    everything else  -> 400

Environment variables set:
  CRE_CLI_GRAPHQL_URL = mock server URL + "/graphql"
  CRE_API_KEY = "test-api"
  CRE_ETH_PRIVATE_KEY = test private key (for simulation)
```

This follows the exact pattern from `test/init_and_simulate_ts_test.go`.

### 3.4 Expected Runtime

| Template | Init | Build/Install | Simulate | Total |
|----------|------|---------------|----------|-------|
| Go PoR (1) | ~10s | ~30s (go build + go get) | ~30s | ~70s |
| Go HelloWorld (2) | ~10s | ~20s | ~15s | ~45s |
| TS HelloWorld (3) | ~5s | ~10s (bun install) | ~15s | ~30s |
| TS PoR (4) | ~5s | ~10s | ~20s | ~35s |
| TS ConfHTTP (5) | ~5s | ~10s | ~15s | ~30s |
| **Total** | | | | **~3.5 min** |

With Go module cache warming and parallel sub-tests: estimated **2-4 minutes** per platform.

---

## 4. SDK Version Matrix Job

### 4.1 Purpose

Detect when a new SDK release breaks existing templates, BEFORE users encounter the issue. This runs on a schedule and can be triggered by SDK release events.

### 4.2 Workflow Specification

```yaml
# New file: .github/workflows/sdk-version-matrix.yml

name: SDK Version Matrix
on:
  schedule:
    - cron: '0 6 * * *'  # Daily at 6am UTC
  workflow_dispatch:
    inputs:
      go_sdk_version:
        description: 'Override Go SDK version (e.g., v1.3.0)'
        required: false
      ts_sdk_version:
        description: 'Override TS SDK version (e.g., 1.1.0)'
        required: false
  repository_dispatch:
    types: [sdk-release]

jobs:
  go-sdk-matrix:
    name: "Go SDK ${{ matrix.sdk_version }}"
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        sdk_version:
          - pinned    # use version from go_module_init.go (v1.2.0)
          - latest    # resolve latest release tag from GitHub
        template_id: [1, 2]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'
      - name: Install Foundry
        uses: foundry-rs/foundry-toolchain@v1
        with:
          version: v1.1.0
      - name: Build CLI
        run: make build
      - name: Init Template
        run: |
          ./cre init -p test-project -t ${{ matrix.template_id }} -w test-wf \
            ${{ matrix.template_id == 1 && '--rpc-url https://ethereum-sepolia-rpc.publicnode.com' || '' }}
      - name: Override SDK Version
        if: matrix.sdk_version != 'pinned'
        run: |
          cd test-project
          # Resolve latest version
          SDK_VERSION=$(go list -m -versions github.com/smartcontractkit/cre-sdk-go | tr ' ' '\n' | tail -1)
          go get github.com/smartcontractkit/cre-sdk-go@${SDK_VERSION}
          go mod tidy
      - name: Build
        run: cd test-project && go build ./...
      - name: Simulate
        run: |
          cd test-project
          CRE_API_KEY=test-api ./cre workflow simulate test-wf \
            --non-interactive --trigger-index=0
        env:
          CRE_CLI_GRAPHQL_URL: "http://localhost:0/graphql"  # mock needed

  ts-sdk-matrix:
    name: "TS SDK ${{ matrix.sdk_version }}"
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        sdk_version:
          - pinned    # use version from package.json.tpl (^1.0.9)
          - latest    # resolve latest from npm
        template_id: [3, 4, 5]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'
      - uses: oven-sh/setup-bun@v2
        with:
          bun-version-file: '.tool-versions'
      - name: Install Foundry
        uses: foundry-rs/foundry-toolchain@v1
        with:
          version: v1.1.0
      - name: Build CLI
        run: make build
      - name: Init Template
        run: |
          ./cre init -p test-project -t ${{ matrix.template_id }} -w test-wf \
            ${{ matrix.template_id == 4 && '--rpc-url https://ethereum-sepolia-rpc.publicnode.com' || '' }}
      - name: Override SDK Version
        if: matrix.sdk_version != 'pinned'
        run: |
          cd test-project/test-wf
          LATEST=$(npm view @chainlink/cre-sdk version)
          bun add @chainlink/cre-sdk@${LATEST}
      - name: Install
        run: cd test-project/test-wf && bun install
      - name: Simulate
        run: |
          cd test-project
          CRE_API_KEY=test-api ./cre workflow simulate test-wf \
            --non-interactive --trigger-index=0
        env:
          CRE_CLI_GRAPHQL_URL: "http://localhost:0/graphql"  # mock needed
```

### 4.3 Cross-Repository Trigger

When the `cre-sdk-go` or `@chainlink/cre-sdk` repositories publish a new release, they should trigger this matrix test in the `cre-cli` repository.

**Option A: GitHub repository_dispatch**

In the SDK repository's release workflow:
```yaml
- name: Trigger CRE CLI compatibility test
  uses: peter-evans/repository-dispatch@v3
  with:
    token: ${{ secrets.CROSS_REPO_TOKEN }}
    repository: smartcontractkit/cre-cli
    event-type: sdk-release
    client-payload: '{"sdk": "cre-sdk-go", "version": "${{ github.ref_name }}"}'
```

**Option B: GitHub Actions webhook via npm publish**

For the TypeScript SDK published to npm, use a GitHub Action that monitors npm for new versions and triggers a dispatch.

**Option C: Scheduled polling (simplest)**

The nightly cron (`0 6 * * *`) checks the latest SDK versions and runs the matrix. This has a 24-hour detection delay but requires no cross-repo setup.

### 4.4 Matrix Dimensions

| Dimension | Values | Source |
|----------|--------|--------|
| Go SDK version | pinned (v1.2.0), latest release, latest pre-release | `go list -m -versions` |
| TS SDK version | pinned (^1.0.9 resolved), latest npm release | `npm view` |
| Go template | 1 (PoR), 2 (HelloWorld) | Template registry |
| TS template | 3 (HelloWorld), 4 (PoR), 5 (ConfHTTP) | Template registry |
| OS | ubuntu-latest (nightly), + windows/macOS (pre-release) | CI matrix |

**Full matrix size**: 2 Go SDK versions x 2 Go templates + 2 TS SDK versions x 3 TS templates = 4 + 6 = **10 jobs per OS**.

---

## 5. AI Agent Integration Job

### 5.1 When to Run

The AI agent integration runs in situations where interpretive testing adds value beyond scripts:

- **Pre-release**: Before publishing a new version (tag push to `v*`)
- **On-demand**: Manual trigger for investigation or ad-hoc validation
- **After SDK compatibility failures**: When the SDK matrix job fails, the AI agent can diagnose the issue

The AI agent is NOT in the critical path for PR merges -- it is advisory.

### 5.2 Workflow Specification

```yaml
# New file: .github/workflows/ai-validation.yml

name: AI-Augmented Validation
on:
  workflow_dispatch:
    inputs:
      scope:
        description: 'Test scope'
        required: true
        default: 'templates'
        type: choice
        options:
          - templates
          - full-journey
      binary_artifact:
        description: 'Use pre-built binary from artifact (run ID)'
        required: false
  push:
    tags:
      - 'v*'

jobs:
  ai-template-validation:
    name: "AI Template Validation"
    runs-on: ubuntu-latest
    timeout-minutes: 60
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'

      - uses: oven-sh/setup-bun@v2
        with:
          bun-version-file: '.tool-versions'

      - name: Install Foundry
        uses: foundry-rs/foundry-toolchain@v1
        with:
          version: v1.1.0

      - name: Build CLI
        run: make build

      - name: Run AI Validation Agent
        run: |
          # The AI agent is invoked here.
          # Implementation depends on the chosen AI tool:
          #
          # Option A: Claude Code CLI
          #   claude-code --prompt-file .ai-test-agent/template-validation.md \
          #     --output .qa-test-report-$(date +%Y-%m-%d).md
          #
          # Option B: Custom wrapper script
          #   ./scripts/run-ai-validation.sh --scope ${{ inputs.scope }}
          #
          # Option C: Direct API call to Claude
          #   .ai-test-agent/run.sh
          #
          # The agent has access to the CLI binary, all tools, and the runbook.
          echo "AI agent execution placeholder"
        env:
          CRE_API_KEY: ${{ secrets.CRE_API_KEY_TEST }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Upload Test Report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: ai-test-report-${{ github.sha }}
          path: .qa-test-report-*.md

      - name: Post Report Summary
        if: always()
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const reports = fs.readdirSync('.').filter(f => f.startsWith('.qa-test-report-'));
            if (reports.length > 0) {
              const content = fs.readFileSync(reports[0], 'utf8');
              // Extract summary table
              const summaryMatch = content.match(/## Summary[\s\S]*?\n\n/);
              if (summaryMatch) {
                core.summary.addRaw(summaryMatch[0]);
                await core.summary.write();
              }
            }
```

### 5.3 AI Agent Invocation Patterns

There are several ways to invoke an AI agent in CI. The choice depends on the team's tooling preferences:

**Pattern A: Claude Code CLI**

```bash
# Claude Code runs as a CLI tool with access to the filesystem
claude-code \
  --system-prompt "$(cat .ai-test-agent/system-prompt.md)" \
  --prompt "Run template compatibility tests for all 5 templates. \
            Build the CLI with 'make build'. Use CRE_API_KEY for auth. \
            Generate a report to .qa-test-report-$(date +%Y-%m-%d).md" \
  --timeout 3600 \
  --working-dir .
```

**Pattern B: Structured Script with AI Interpretation**

```bash
# Run deterministic script first, then have AI interpret results
go test -v -json -timeout 20m -run TestTemplateCompatibility ./test/ > test-results.json

# AI interprets results and generates report
claude-code \
  --system-prompt "$(cat .ai-test-agent/report-generator.md)" \
  --prompt "Analyze test-results.json and generate a human-readable report. \
            For any failures, diagnose the root cause by reading the error output."
```

**Pattern C: Hybrid (Recommended)**

```bash
# Phase 1: Script runs tests, captures output
./scripts/template-test.sh > test-output.log 2>&1
TEST_EXIT_CODE=$?

# Phase 2: AI analyzes output and generates report
claude-code \
  --system-prompt "$(cat .ai-test-agent/analyzer.md)" \
  --prompt "Analyze test-output.log (exit code: $TEST_EXIT_CODE). \
            Generate a structured test report. If there are failures, \
            read the relevant source files to diagnose the root cause."
```

Pattern C is recommended because it separates the deterministic execution (reproducible, fast) from the interpretive analysis (AI, adds insight). If the AI fails or times out, the script results are still available.

---

## 6. Cross-Repository Triggers

### 6.1 SDK Release Detection

```
cre-sdk-go repository                     cre-cli repository
+----------------------------+             +----------------------------+
|                            |             |                            |
| Release v1.3.0 published   |             |                            |
|     |                      |             |                            |
|     v                      |             |                            |
| release.yml:               |  dispatch   | sdk-version-matrix.yml:    |
|   repository_dispatch -----+------------>|   go-sdk-matrix:           |
|   event: sdk-release       |             |     sdk_version: v1.3.0    |
|   payload:                 |             |     template_id: [1, 2]    |
|     sdk: cre-sdk-go        |             |                            |
|     version: v1.3.0        |             |   ts-sdk-matrix:           |
|                            |             |     [skipped for Go SDK]   |
+----------------------------+             +----------------------------+
```

```
@chainlink/cre-sdk (npm)                   cre-cli repository
+----------------------------+             +----------------------------+
|                            |             |                            |
| npm publish v1.1.0         |             |                            |
|     |                      |  dispatch   | sdk-version-matrix.yml:    |
|     v (via npm hook or     +------------>|   ts-sdk-matrix:           |
|      GitHub Action watcher)|             |     sdk_version: 1.1.0     |
|                            |             |     template_id: [3, 4, 5] |
|                            |             |                            |
+----------------------------+             +----------------------------+
```

### 6.2 Required Secrets

| Secret | Repository | Purpose |
|--------|-----------|---------|
| `CROSS_REPO_TOKEN` | SDK repos | PAT with `repo` scope to trigger dispatches in cre-cli |
| `CRE_API_KEY_TEST` | cre-cli | Test environment API key for AI validation |
| `ANTHROPIC_API_KEY` | cre-cli | Claude API key for AI agent (if using API directly) |

### 6.3 Notification on Failure

When the SDK matrix job fails:

```yaml
- name: Notify on SDK Compatibility Failure
  if: failure()
  uses: slackapi/slack-github-action@v1
  with:
    payload: |
      {
        "text": "SDK compatibility failure detected",
        "blocks": [
          {
            "type": "section",
            "text": {
              "type": "mrkdwn",
              "text": "*SDK Version Matrix Failed*\nSDK: ${{ matrix.sdk_version }}\nTemplate: ${{ matrix.template_id }}\n<${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}|View Run>"
            }
          }
        ]
      }
  env:
    SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK }}
```

---

## 7. Environment and Secrets Management

### 7.1 Environments

| Environment | Used By | Purpose |
|------------|---------|---------|
| CI (no env) | Template compatibility, SDK matrix | Mocked services, no real API calls |
| STAGING | AI validation (pre-release) | Real APIs with test data |
| PRODUCTION | Manual QA only | Real APIs with real data |

### 7.2 Secrets Inventory

| Secret | Scope | Rotation | Used By |
|--------|-------|----------|---------|
| `CRE_API_KEY_TEST` | STAGING env | 90 days | AI validation job |
| `ANTHROPIC_API_KEY` | CI | Per provider policy | AI agent invocation |
| `CROSS_REPO_TOKEN` | SDK repos -> cre-cli | 90 days | Repository dispatch |
| `SLACK_WEBHOOK` | cre-cli | As needed | Failure notifications |
| `ETH_PRIVATE_KEY_TEST` | STAGING env | Per wallet | On-chain operations (future) |

### 7.3 Credential Safety

- All on-chain operations use STAGING/testnet only -- never mainnet credentials in CI
- API keys are scoped to test organizations with no production data access
- Private keys are for dedicated test wallets with small testnet ETH balances
- All secrets are stored in GitHub Secrets (encrypted at rest)
- No credentials are logged or included in test artifacts

### 7.4 Playwright Credential Bootstrap (Proposal-Only)

- Playwright-based browser credential bootstrap is an **optional local proposal** for unblocking diagnostic runs.
- It is **not** a baseline requirement and **not** a CI-default merge gate in this framework.
- If bootstrap is unavailable, credential-dependent tests should be reported as `BLOCKED_AUTH` rather than treated as deterministic failures.

---

## 8. Cost and Runtime Analysis

### 8.1 CI Runner Costs

| Job | Runner | Est. Runtime | Frequency | Monthly Runs | Monthly Cost* |
|-----|--------|-------------|-----------|-------------|---------------|
| template-compat (Ubuntu) | ubuntu-latest | 4 min | Every PR (~50/mo) | 50 | ~$2 |
| template-compat (Windows) | windows-latest | 6 min | Every PR (~50/mo) | 50 | ~$10 |
| template-compat (macOS) | macos-latest | 5 min | Release PRs (~5/mo) | 5 | ~$5 |
| sdk-matrix (10 jobs) | ubuntu-latest | 30 min total | Daily + SDK releases | 35 | ~$7 |
| ai-validation | ubuntu-latest | 45 min | Pre-release (~4/mo) | 4 | ~$3 |

*Based on GitHub Actions pricing: Linux $0.008/min, Windows $0.016/min, macOS $0.08/min

**Total estimated monthly CI cost: ~$27**

### 8.2 AI Agent Costs

| Operation | Tokens (est.) | Cost per Run* | Frequency | Monthly Cost |
|-----------|--------------|---------------|-----------|--------------|
| Template validation (5 templates) | ~50K input + 10K output | ~$3 | 4/month | ~$12 |
| Full journey validation | ~150K input + 30K output | ~$10 | 2/month | ~$20 |
| Failure diagnosis (ad-hoc) | ~30K input + 5K output | ~$2 | 3/month | ~$6 |

*Based on Claude API pricing; varies by model selection

**Total estimated monthly AI cost: ~$38**

### 8.3 Total Cost

| Category | Monthly Cost |
|----------|-------------|
| CI runners | ~$27 |
| AI agent | ~$38 |
| **Total** | **~$65** |

This is significantly less than the cost of one engineer spending 2-4 hours on manual QA per release (~$200-400 in engineer time at loaded cost).

---

## 9. Rollout Plan

### Phase 1: Template Compatibility Tests (Week 1-2)

**Goal**: Get all 5 templates tested in CI on every PR.

**Steps**:
1. Write `test/template_compatibility_test.go`
2. Add `ci-test-template-compat` job to `pull-request-main.yml`
3. Run on Ubuntu + Windows matrix (match existing E2E)
4. Validate all 5 templates pass
5. Merge as a required check

**Risk**: Low. This is additive -- does not change existing tests.

**Validation**: Deliberately break a template (e.g., rename a function in `main.go.tpl`) and verify CI catches it.

### Phase 2: SDK Version Matrix (Week 2-3)

**Goal**: Detect SDK breakage within 24 hours.

**Steps**:
1. Create `sdk-version-matrix.yml` workflow
2. Set up nightly cron schedule
3. Configure Slack notification on failure
4. Run first successful nightly pass
5. Set up cross-repo dispatch from SDK repos (if access is granted)

**Risk**: Medium. Depends on SDK team cooperation for repository_dispatch. Nightly cron is a fallback that requires no cross-repo setup.

### Phase 3: AI Agent PoC (Week 3-4)

**Goal**: Demonstrate AI-augmented testing works in CI.

**Steps**:
1. Write AI agent prompt/instructions (CLAUDE.md or equivalent)
2. Test locally with Claude Code CLI
3. Add `ai-validation.yml` workflow (manual trigger only)
4. Run first successful report generation
5. Review report quality with team

**Risk**: Medium-High. AI agent behavior may need iteration. Start with `workflow_dispatch` only -- do not automate until validated.

### Phase 4: macOS and Full Integration (Week 4-5)

**Goal**: Complete platform coverage and automate AI runs.

**Steps**:
1. Add macOS runner to template-compat job (behind label gate or release PRs only)
2. Enable AI validation on tag push (pre-release automation)
3. Set up cross-repo triggers from SDK repositories
4. Document the full workflow for the CRE team

**Risk**: Low for macOS addition. Medium for cross-repo triggers (requires coordination).

### Phase 5: Handoff (Week 5-6)

**Goal**: CRE team owns and maintains the testing framework.

**Steps**:
1. Knowledge transfer session: how to add new templates to tests
2. Knowledge transfer session: how to read and act on AI reports
3. Document maintenance procedures (secret rotation, runner updates, agent prompt updates)
4. CRE team runs their first independent test cycle
5. We remain available for questions during the first month

---

## 10. Monitoring and Alerting

### 10.1 What to Monitor

| Signal | Source | Alert Threshold |
|--------|--------|----------------|
| Template compatibility failures | `ci-test-template-compat` job | Any failure (PR blocker) |
| SDK matrix failures | `sdk-version-matrix` nightly | Any failure (Slack alert) |
| AI validation failures | `ai-validation` report | FAIL in summary (PR comment) |
| Nightly job not running | Cron schedule | Missing run for 48 hours |
| CI runtime increase | All jobs | >2x baseline runtime |
| AI agent timeout | `ai-validation` job | Exceeds 60-minute timeout |

### 10.2 Alert Destinations

```
Template compatibility failure (PR):
  -> GitHub Check (blocks merge)
  -> PR comment with failure details

SDK matrix failure (nightly):
  -> Slack channel (#cre-cli-alerts)
  -> GitHub issue (auto-created)

AI validation failure (pre-release):
  -> Report artifact uploaded to Actions
  -> PR comment with summary table
  -> Slack channel (for visibility)

Cross-repo trigger failure:
  -> Slack channel
  -> SDK team notified
```

### 10.3 Dashboard

A simple GitHub Actions dashboard provides visibility:

- **Template Compatibility**: badge in README showing latest status
- **SDK Matrix**: badge showing nightly status (green/red/yellow)
- **AI Validation**: link to latest report artifact

```markdown
<!-- In README.md -->
![Template Tests](https://github.com/smartcontractkit/cre-cli/actions/workflows/pull-request-main.yml/badge.svg)
![SDK Matrix](https://github.com/smartcontractkit/cre-cli/actions/workflows/sdk-version-matrix.yml/badge.svg)
```
