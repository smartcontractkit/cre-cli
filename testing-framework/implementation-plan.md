# Implementation Plan: CRE CLI Template Testing

> A concrete plan to solve template breakage, close testing gaps, and position AI for pre-release validation across current embedded templates and upcoming branch-gated dynamic template pulls. Audience: PM, CRE engineering, and our team.

---

## 1. Problem Summary

Templates are the primary entry point for CRE developers. The CLI currently ships 5 embedded templates, but only 3 have automated test coverage. A dynamic template pull model is planned and introduces additional compatibility risk across CLI and template-repo versions. When the Go SDK (`cre-sdk-go`) or TypeScript SDK (`@chainlink/cre-sdk`) releases a new version, nothing validates that existing templates still compile and simulate. Breakage reaches users before it reaches the team.

The dependency chain that breaks silently:

```
CLI binary (embeds templates at build time)
  |
  +-- Go templates import cre-sdk-go
  |   Version pinned in cmd/creinit/go_module_init.go:
  |     SdkVersion              = "v1.2.0"
  |     EVMCapabilitiesVersion  = "v1.0.0-beta.5"
  |     HTTPCapabilitiesVersion = "v1.0.0-beta.0"
  |     CronCapabilitiesVersion = "v1.0.0-beta.0"
  |
  +-- TS templates declare @chainlink/cre-sdk in package.json.tpl:
      "@chainlink/cre-sdk": "^1.0.9"   <-- caret range, resolved at user's bun install time
      "viem": "2.34.0"
      "zod": "3.25.76"
```

The Go side uses exact pins (safe but stale). The TypeScript side uses a caret range, meaning a new `@chainlink/cre-sdk` minor release can break every TS template for every user without any CLI change.

Evidence from the Windows QA report (2026-02-12): Claude Code executed the full runbook against the preview binary and found 1 bug (invalid URL scheme accepted), 5 runbook discrepancies, and 2 non-blocking issues. All were detectable by automated tests.

---

## 2. Assessment of Current Testing

### What exists

| Layer | Coverage | Location |
|-------|----------|----------|
| Unit tests | All packages except `usbwallet` and `test/` | CI: `ci-test-unit` |
| E2E tests | Templates 1 (Go PoR), 2 (Go HelloWorld), 4 (TS PoR) | CI: `ci-test-e2e` on Ubuntu + Windows |
| Mock infrastructure | GraphQL, Storage, Vault, PoR HTTP -- all via `httptest.Server` | `test/multi_command_flows/` |
| Anvil state | Pre-baked state for on-chain simulation | `test/anvil-state.json` |
| System tests | Full OCR3 PoR against Chainlink infra | **DISABLED** (`if: false`) |
| Manual QA | 103-test runbook, 2-4 hours per run | `.qa-developer-runbook.md` |

### What is missing

| Gap | Impact |
|-----|--------|
| Template 3 (TS HelloWorld) -- zero test coverage | Breakage undetected |
| Template 5 (TS ConfHTTP) -- zero test coverage | Breakage undetected |
| No SDK version matrix testing | SDK releases break templates silently |
| No macOS CI runner | Platform-specific bugs missed |
| Interactive wizard flows (18 tests) | Skipped in all automated runs |
| Real-service validation | API contract changes slip through |

### Key insight

The E2E test infrastructure is solid. The existing test at `test/init_and_simulate_ts_test.go` already demonstrates the exact pattern needed: init template with flags, `bun install`, simulate with `--non-interactive`, assert success. The gap is coverage, not architecture.

---

## 3. Implementation Plan

### Dynamic-Template Branch Gate

- Add a dynamic-source compatibility harness deliverable once branch/repo links are available.
- Run dynamic-source checks as advisory first.
- Promote to required CI gate only after branch-level flake rate and stability are acceptable.

### Merge Gate Policy and Operational Reporting Contract

Default enforcement model for this plan:

- **Required merge gates:** deterministic checks only (template compatibility plus deterministic smoke/negative-path checks).
- **Advisory by default:** large exploratory AI/nightly runs unless explicitly promoted to required by team policy.
- **Manual/browser checks:** non-gating and tracked as manual-signoff evidence.

Operational report status vocabulary:

- `PASS`
- `FAIL`
- `SKIP`
- `BLOCKED`

Standard reason codes:

- `BLOCKED_ENV`, `BLOCKED_AUTH`
- `FAIL_COMPAT`, `FAIL_TUI`, `FAIL_NEGATIVE_PATH`, `FAIL_CONTRACT`
- `SKIP_MANUAL`, `SKIP_PLATFORM`

### Deliverable 1: Template Compatibility Test

**What**: A single Go test file that exercises init + build + simulate for all 5 templates. Data-driven, so adding template 6 is a one-line table entry.

**PM concern addressed**: "templates break silently" + "template library is about to grow significantly"

**Effort**: 1-2 days

**File**: `test/template_compatibility_test.go`

**Design**: Data-driven test table mirroring the template registry in `cmd/creinit/creinit.go`:

```go
var templateTests = []struct {
    name         string
    templateID   string
    lang         string // "go" or "ts"
    needsRpcUrl  bool
    expectedFiles []string
    simulateCheck string // substring expected in simulate output
}{
    {
        name:         "Go_PoR_Template1",
        templateID:   "1",
        lang:         "go",
        needsRpcUrl:  true,
        expectedFiles: []string{"main.go", "workflow.go", "workflow_test.go", "workflow.yaml"},
        simulateCheck: "Workflow compiled",
    },
    {
        name:         "Go_HelloWorld_Template2",
        templateID:   "2",
        lang:         "go",
        needsRpcUrl:  false,
        expectedFiles: []string{"main.go", "workflow.yaml"},
        simulateCheck: "Workflow compiled",
    },
    {
        name:         "TS_HelloWorld_Template3",
        templateID:   "3",
        lang:         "ts",
        needsRpcUrl:  false,
        expectedFiles: []string{"main.ts", "package.json", "tsconfig.json", "workflow.yaml"},
        simulateCheck: "Workflow compiled",
    },
    {
        name:         "TS_PoR_Template4",
        templateID:   "4",
        lang:         "ts",
        needsRpcUrl:  true,
        expectedFiles: []string{"main.ts", "package.json", "tsconfig.json", "workflow.yaml"},
        simulateCheck: "Workflow compiled",
    },
    {
        name:         "TS_ConfHTTP_Template5",
        templateID:   "5",
        lang:         "ts",
        needsRpcUrl:  false,
        expectedFiles: []string{"main.ts", "package.json", "tsconfig.json", "workflow.yaml"},
        simulateCheck: "Workflow compiled",
    },
}
```

**Test flow per template** (follows existing pattern from `test/init_and_simulate_ts_test.go`):

```go
func TestTemplateCompatibility(t *testing.T) {
    for _, tt := range templateTests {
        t.Run(tt.name, func(t *testing.T) {
            tempDir := t.TempDir()
            projectName := "compat-" + tt.templateID
            workflowName := "test-wf"
            projectRoot := filepath.Join(tempDir, projectName)
            workflowDir := filepath.Join(projectRoot, workflowName)

            // Set env (same as existing E2E tests)
            t.Setenv(settings.EthPrivateKeyEnvVar, testPrivateKey)
            t.Setenv(credentials.CreApiKeyVar, "test-api")

            // Mock GraphQL (reuse existing pattern)
            gqlSrv := startMockGraphQL(t)
            defer gqlSrv.Close()
            t.Setenv(environments.EnvVarGraphQLURL, gqlSrv.URL+"/graphql")

            // Step 1: cre init
            initArgs := []string{
                "init",
                "--project-root", tempDir,
                "--project-name", projectName,
                "--template-id", tt.templateID,
                "--workflow-name", workflowName,
            }
            if tt.needsRpcUrl {
                initArgs = append(initArgs, "--rpc-url", constants.DefaultEthSepoliaRpcUrl)
            }
            runCLI(t, initArgs...)

            // Step 2: Verify files
            require.FileExists(t, filepath.Join(projectRoot, "project.yaml"))
            for _, f := range tt.expectedFiles {
                require.FileExists(t, filepath.Join(workflowDir, f))
            }

            // Step 3: Build
            if tt.lang == "go" {
                runCmd(t, projectRoot, "go", "build", "./...")
            } else {
                runCmd(t, workflowDir, "bun", "install")
            }

            // Step 4: Simulate
            output := runCLI(t,
                "workflow", "simulate", workflowName,
                "--project-root", projectRoot,
                "--non-interactive", "--trigger-index=0",
            )
            require.Contains(t, output, tt.simulateCheck)
        })
    }
}
```

Where `startMockGraphQL` is a helper extracted from the existing pattern in `test/init_and_simulate_ts_test.go` (handles `getOrganization` query, returns 400 for everything else), and `runCLI`/`runCmd` are thin wrappers around `exec.Command` that capture output and fail on non-zero exit.

**Canary test** (ensures test table stays in sync with template registry):

```go
func TestTemplateCompatibility_AllTemplatesCovered(t *testing.T) {
    // Count templates in the test table
    testedIDs := make(map[string]bool)
    for _, tt := range templateTests {
        testedIDs[tt.templateID] = true
    }
    // The template registry currently has 5 templates (IDs 1-5).
    // If this assertion fails, a new template was added to
    // cmd/creinit/creinit.go but not to this test table.
    require.Equal(t, 5, len(testedIDs),
        "template count mismatch: update templateTests when adding new templates")
}
```

**CI job** (addition to `.github/workflows/pull-request-main.yml`):

```yaml
ci-test-template-compat:
  runs-on: ${{ matrix.os }}
  strategy:
    fail-fast: false
    matrix:
      os: [ubuntu-latest, windows-latest]
  permissions:
    id-token: write
    contents: read
    actions: read
  steps:
    - name: setup-foundry
      uses: foundry-rs/foundry-toolchain@82dee4ba654bd2146511f85f0d013af94670c4de # v1.4.0
      with:
        version: "v1.1.0"

    - name: Install Bun (Linux)
      if: runner.os == 'Linux'
      run: |
        curl -fsSL https://bun.sh/install | bash
        echo "$HOME/.bun/bin" >> "$GITHUB_PATH"

    - name: Install Bun (Windows)
      if: runner.os == 'Windows'
      shell: pwsh
      run: |
        powershell -c "irm bun.sh/install.ps1 | iex"
        $bunBin = Join-Path $env:USERPROFILE ".bun\bin"
        $bunBin | Out-File -FilePath $env:GITHUB_PATH -Encoding utf8 -Append

    - name: ci-test-template-compat
      uses: smartcontractkit/.github/actions/ci-test-go@2b1d964024bb001ae9fba4f840019ac86ad1d824
      env:
        TEST_LOG_LEVEL: debug
      with:
        go-test-cmd: go test -v -timeout 20m -run TestTemplateCompatibility ./test/
        use-go-cache: "true"
        aws-region: ${{ secrets.AWS_REGION }}
        use-gati: "true"
        aws-role-arn-gati: ${{ secrets.AWS_OIDC_DEV_PLATFORM_READ_REPOS_EXTERNAL_TOKEN_ISSUER_ROLE_ARN }}
        aws-lambda-url-gati: ${{ secrets.AWS_DEV_SERVICES_TOKEN_ISSUER_LAMBDA_URL }}
```

**What it catches**: Any template that fails to init, build, or simulate. Automatically covers new templates when added to the test table. The canary test alerts if the table falls behind the registry.

---

### Deliverable 2: Nightly SDK Version Matrix

**What**: A scheduled GitHub Actions workflow that tests templates against the latest SDK versions, catching breakage within 24 hours of an SDK release.

**PM concern addressed**: "catch it before users complain" for SDK changes

**Effort**: 1-2 days

**File**: `.github/workflows/sdk-version-matrix.yml`

```yaml
name: SDK Version Matrix
on:
  schedule:
    - cron: '0 6 * * *'  # Daily at 6am UTC
  workflow_dispatch:
    inputs:
      go_sdk_override:
        description: 'Go SDK version to test (e.g. v1.3.0). Leave empty for latest.'
        required: false
      ts_sdk_override:
        description: 'TS SDK version to test (e.g. 1.1.0). Leave empty for latest.'
        required: false
  repository_dispatch:
    types: [sdk-release]

jobs:
  resolve-versions:
    runs-on: ubuntu-latest
    outputs:
      go_sdk_latest: ${{ steps.resolve.outputs.go_sdk_latest }}
      ts_sdk_latest: ${{ steps.resolve.outputs.ts_sdk_latest }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'
      - name: Resolve latest SDK versions
        id: resolve
        run: |
          # Go SDK: latest tag from go module proxy
          GO_LATEST=$(go list -m -versions github.com/smartcontractkit/cre-sdk-go 2>/dev/null \
            | tr ' ' '\n' | grep -v alpha | grep -v beta | tail -1)
          echo "go_sdk_latest=${GO_LATEST:-v1.2.0}" >> "$GITHUB_OUTPUT"

          # TS SDK: latest from npm
          TS_LATEST=$(npm view @chainlink/cre-sdk version 2>/dev/null || echo "1.0.9")
          echo "ts_sdk_latest=${TS_LATEST}" >> "$GITHUB_OUTPUT"

  go-templates:
    needs: resolve-versions
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        template_id: ["1", "2"]
        sdk_version: ["pinned", "latest"]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'
      - uses: foundry-rs/foundry-toolchain@v1
        with:
          version: "v1.1.0"

      - name: Build CLI
        run: make build

      - name: Init template
        run: |
          EXTRA_FLAGS=""
          if [ "${{ matrix.template_id }}" = "1" ]; then
            EXTRA_FLAGS="--rpc-url https://ethereum-sepolia-rpc.publicnode.com"
          fi
          ./cre init -p test-project -t ${{ matrix.template_id }} -w test-wf $EXTRA_FLAGS

      - name: Override SDK version (latest)
        if: matrix.sdk_version == 'latest'
        run: |
          cd test-project
          SDK_VER="${{ inputs.go_sdk_override || needs.resolve-versions.outputs.go_sdk_latest }}"
          echo "Overriding Go SDK to ${SDK_VER}"
          go get github.com/smartcontractkit/cre-sdk-go@${SDK_VER}
          go mod tidy

      - name: Build
        run: cd test-project && go build ./...

      - name: Simulate
        run: |
          cd test-project
          CRE_API_KEY=test-api \
          CRE_CLI_GRAPHQL_URL=http://localhost:1/graphql \
          ./cre workflow simulate test-wf \
            --non-interactive --trigger-index=0 || true
          # Note: simulate may fail due to mock GraphQL not being available.
          # The primary validation is that the build step succeeds.
          # Full simulate validation happens in ci-test-template-compat.

  ts-templates:
    needs: resolve-versions
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        template_id: ["3", "4", "5"]
        sdk_version: ["pinned", "latest"]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: '.tool-versions'
      - uses: oven-sh/setup-bun@v2
        with:
          bun-version-file: '.tool-versions'
      - uses: foundry-rs/foundry-toolchain@v1
        with:
          version: "v1.1.0"

      - name: Build CLI
        run: make build

      - name: Init template
        run: |
          EXTRA_FLAGS=""
          if [ "${{ matrix.template_id }}" = "4" ]; then
            EXTRA_FLAGS="--rpc-url https://ethereum-sepolia-rpc.publicnode.com"
          fi
          ./cre init -p test-project -t ${{ matrix.template_id }} -w test-wf $EXTRA_FLAGS

      - name: Override SDK version (latest)
        if: matrix.sdk_version == 'latest'
        run: |
          cd test-project/test-wf
          TS_VER="${{ inputs.ts_sdk_override || needs.resolve-versions.outputs.ts_sdk_latest }}"
          echo "Overriding TS SDK to ${TS_VER}"
          bun add @chainlink/cre-sdk@${TS_VER}

      - name: Install
        run: cd test-project/test-wf && bun install

  notify-on-failure:
    needs: [go-templates, ts-templates]
    if: failure()
    runs-on: ubuntu-latest
    steps:
      - name: Notify Slack
        uses: slackapi/slack-github-action@v1
        with:
          payload: |
            {
              "text": "SDK Version Matrix FAILED - templates may be broken with latest SDK versions. <${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}|View Run>"
            }
        env:
          SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

**Cross-repo trigger** (for the SDK team to add to their release workflow):

```yaml
# In cre-sdk-go release workflow:
- name: Trigger CRE CLI compatibility check
  uses: peter-evans/repository-dispatch@v3
  with:
    token: ${{ secrets.CROSS_REPO_PAT }}
    repository: smartcontractkit/cre-cli
    event-type: sdk-release
    client-payload: '{"sdk": "cre-sdk-go", "version": "${{ github.ref_name }}"}'
```

If cross-repo triggers are not feasible immediately, the nightly cron provides the same coverage with up to a 24-hour delay.

**What it catches**: SDK version N+1 breaks templates. Detected within 24 hours (nightly) or immediately (cross-repo trigger).

---

### Deliverable 3: PTY Test Wrapper for Interactive Flows

**What**: A Go test helper using `github.com/creack/pty` that spawns the CLI in a pseudo-terminal, enabling automated testing of the Bubbletea wizard and interactive prompts.

**PM concern addressed**: "full user journey coverage" -- the 18 tests currently SKIPped because they require TTY

**Effort**: 2-3 days

**File**: `test/pty_helper_test.go` + test cases in `test/wizard_test.go`

**PTY helper design**:

```go
// ptySession wraps a CLI process running in a pseudo-terminal.
type ptySession struct {
    pty    *os.File
    cmd    *exec.Cmd
    output *bytes.Buffer
}

// startPTY launches the CLI binary in a pseudo-terminal.
func startPTY(t *testing.T, args ...string) *ptySession {
    cmd := exec.Command(CLIPath, args...)
    ptmx, err := pty.Start(cmd)
    require.NoError(t, err)
    t.Cleanup(func() {
        ptmx.Close()
        cmd.Process.Kill()
    })
    buf := &bytes.Buffer{}
    go io.Copy(buf, ptmx) // background reader
    return &ptySession{pty: ptmx, cmd: cmd, output: buf}
}

// waitFor reads output until the given pattern appears or timeout.
func (s *ptySession) waitFor(t *testing.T, pattern string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if strings.Contains(StripANSI(s.output.String()), pattern) {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
    t.Fatalf("timed out waiting for %q in output:\n%s", pattern, s.output.String())
}

// send writes keystrokes to the PTY.
func (s *ptySession) send(input string) {
    s.pty.Write([]byte(input))
}

// sendKey sends a special key (arrow, Esc, Enter, Ctrl+C).
func (s *ptySession) sendKey(key string) {
    keys := map[string]string{
        "enter":  "\r",
        "esc":    "\x1b",
        "ctrl-c": "\x03",
        "up":     "\x1b[A",
        "down":   "\x1b[B",
    }
    s.pty.Write([]byte(keys[key]))
}
```

**Example wizard test**:

```go
func TestWizard_FullFlow(t *testing.T) {
    s := startPTY(t, "init")

    // Step 1: Project name
    s.waitFor(t, "Project name", 10*time.Second)
    s.send("test-project\r")

    // Step 2: Language selection
    s.waitFor(t, "Language", 5*time.Second)
    s.sendKey("down") // select TypeScript
    s.sendKey("enter")

    // Step 3: Template selection
    s.waitFor(t, "Template", 5*time.Second)
    s.sendKey("enter") // select first template

    // Step 4: Workflow name
    s.waitFor(t, "Workflow name", 5*time.Second)
    s.send("my-wf\r")

    // Verify success
    s.waitFor(t, "Project created successfully", 15*time.Second)
}

func TestWizard_EscCancels(t *testing.T) {
    s := startPTY(t, "init")
    s.waitFor(t, "Project name", 10*time.Second)
    s.sendKey("esc")
    s.waitFor(t, "cancelled", 5*time.Second)
}

func TestWizard_InvalidNameShowsError(t *testing.T) {
    s := startPTY(t, "init")
    s.waitFor(t, "Project name", 10*time.Second)
    s.send("my project!\r") // invalid: contains space and !
    s.waitFor(t, "invalid", 5*time.Second)
}
```

**Note**: PTY tests only run on Unix (Linux/macOS). Windows has different PTY semantics. This is acceptable because the wizard rendering is identical across platforms -- the test validates logic, not visual rendering.

**What it catches**: Wizard navigation bugs, validation feedback issues, Esc/Ctrl+C handling, default value behavior. Covers the 18 tests marked SKIP in the Windows QA report.

---

### Deliverable 4: macOS CI Runner

**What**: Add `macos-latest` to the template compatibility test matrix.

**PM concern addressed**: "multi-platform support (macOS, Windows, Linux)"

**Effort**: 0.5 day

**Change**: Add to the `ci-test-template-compat` matrix in `pull-request-main.yml`:

```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
```

macOS runners cost 10x more than Linux ($0.08/min vs $0.008/min). To manage cost:

- **Option A**: Run macOS on all PRs (adds ~$40/month at 50 PRs)
- **Option B**: Run macOS only on release PRs (label-gated, adds ~$4/month)
- **Option C**: Run macOS in the nightly SDK matrix only (adds ~$2/month)

Option B is recommended. Add a condition:

```yaml
os:
  - ubuntu-latest
  - windows-latest
  - ${{ (github.base_ref == 'main' && contains(github.event.pull_request.labels.*.name, 'release')) && 'macos-latest' || '' }}
```

Or simpler: add macOS to the nightly SDK matrix workflow where the cost is fixed regardless of PR volume.

**What it catches**: Platform-specific path handling, toolchain compatibility, and binary behavior differences on macOS.

---

### Deliverable 5: AI Pre-Release QA Skill

**What**: A Cursor skill that wraps the QA runbook into an executable AI workflow. Invoked before releases to run the full validation suite and produce a structured test report.

**PM concern addressed**: "leverage AI (e.g., Claude Code) to automate validation"

**Effort**: 1 day

**File**: `.cursor/skills/cre-qa-runner/SKILL.md`

```markdown
---
name: cre-qa-runner
description: >
  Run the CRE CLI QA test suite and generate a structured test report.
  Use when preparing a release, after major changes, or when the user
  asks to "run QA", "test the CLI", or "validate templates".
---

# CRE CLI QA Runner

## Prerequisites

Before running, verify tools are available:
- `cre` binary (run `make build` or use pre-built)
- `go version` (need 1.25.5+)
- `bun --version` (need 1.2.21+)
- `node --version` (need 20.13.1+)
- `anvil --version` (need v1.1.0)

Record all version numbers for the report.

## Environment Setup

Set these before running:
- `CRE_API_KEY` -- required for auth-gated commands (get from team)
- `CRE_ETH_PRIVATE_KEY` -- testnet key only, for simulation

## Test Execution

### Phase 1: Smoke Tests (all script-automatable)
Run every `--help` command and verify output. Run `cre version`.
Check exit codes. Record any failures.

### Phase 2: Template Compatibility (all 5 templates)
For each template ID (1-5):
1. `cre init -p test-tpl-<ID> -t <ID> -w test-wf [--rpc-url if PoR]`
2. Verify expected files exist
3. Build: `go build ./...` (Go) or `bun install` (TS)
4. `cre workflow simulate test-wf --non-interactive --trigger-index=0`
5. Record PASS/FAIL with output evidence

Template reference:
| ID | Language | Name | Needs --rpc-url |
|----|----------|------|-----------------|
| 1 | Go | PoR | Yes |
| 2 | Go | HelloWorld | No |
| 3 | TypeScript | HelloWorld | No |
| 4 | TypeScript | PoR | Yes |
| 5 | TypeScript | ConfHTTP | No |

### Phase 3: Edge Cases
Run the negative tests from the runbook (invalid names, missing args, etc.).
These are all exit-code checks.

### Phase 4: Deploy Lifecycle (if staging credentials available)
Deploy -> Pause -> Activate -> Delete with a Go HelloWorld workflow.
Verify each transaction confirms. Record TX hashes.

## Error Handling
- If a template fails, record the failure and continue to the next template.
- If build fails, still attempt simulate (may give a different error).
- Never abort the entire run because of one failure.

## Report Format

Generate the report matching `.qa-test-report-template.md`. Include:
- Run metadata (date, OS, versions, branch, commit)
- Per-template results table
- Summary table with PASS/FAIL/SKIP counts
- Bugs found section
- Recommendations section

Write the report to `.qa-test-report-YYYY-MM-DD.md` in the repo root.
```

**What the AI handles** (10 tests where it adds value over scripts):
- Full deploy lifecycle against real staging services with variable timing
- Error diagnosis when real services return unexpected responses
- `cre update` behavior interpretation across platforms
- Generating a human-readable report with analysis and recommendations
- Adaptive execution when dependencies between steps fail

**What stays manual** (8 tests):
- Browser OAuth login flow
- CRE logo rendering (visual)
- Color visibility on dark/light backgrounds (visual)
- Selection highlighting (visual)
- Error message colors (visual)
- Cross-terminal rendering parity (visual)

---

## 4. What AI Solves vs. Scripts vs. Humans

| Tier | Count | Percentage | What it covers | Example |
|------|-------|------------|----------------|---------|
| **Script** | 99 | 85% | Exit code checks, string matching, file existence, PTY automation via `expect`/`creack/pty` | `cre init -t 2` exits 0, `project.yaml` exists |
| **AI** | 10 | 8% | Real-service interaction, semantic output interpretation, error diagnosis, report generation | Deploy lifecycle against staging, diagnose "insufficient gas" error |
| **Manual** | 8 | 7% | Visual rendering, browser OAuth, subjective UX assessment | "CRE logo renders correctly", "colors visible on dark background" |

The original ask was about leveraging AI. The honest answer: AI adds real value for pre-release QA (replacing a 2-4 hour manual session with a 30-minute AI run). But the core fix for "templates break silently" is a test file -- Deliverable 1 -- which is pure Go code with no AI involvement.

The AI skill (Deliverable 5) is the pre-release thoroughness layer. The CI tests (Deliverables 1-4) are the safety net.

---

## 5. Scaling Strategy

### Auto-discovery

The template compatibility test uses a data-driven table. When a new template is added:
1. Add one entry to `templateTests` in `test/template_compatibility_test.go`
2. Update the canary count from 5 to 6

If someone forgets step 1, the canary test fails CI:

```
template count mismatch: update templateTests when adding new templates
```

For true auto-discovery (no manual step), export `languageTemplates` in `cmd/creinit/creinit.go` (rename to `LanguageTemplates`) and have the test iterate over it directly. This is a one-line change to production code.

### "Add template" skill

A second Cursor skill at `.cursor/skills/cre-add-template/SKILL.md` that guides developers through the full template creation checklist:

1. Create template files in `cmd/creinit/template/workflow/<folder>/`
2. Add entry to `languageTemplates` in `cmd/creinit/creinit.go`
3. Add SDK version pins (Go) or package.json deps (TS)
4. Add entry to `templateTests` in `test/template_compatibility_test.go`
5. Update canary count
6. Run `make test-e2e` to verify
7. Update docs

This prevents the "forgot to add a test" problem that created the current gap.

### SDK version pinning recommendation

Lock TS templates to exact versions to prevent surprise breakage:

```json
// Current (risky):
"@chainlink/cre-sdk": "^1.0.9"

// Recommended:
"@chainlink/cre-sdk": "1.0.9"
```

With exact pins, TS templates behave like Go templates: the version is controlled and updated deliberately. The nightly SDK matrix still tests against latest to detect when an update is needed.

---

## 6. Handoff and Ownership

| What | We deliver | CRE team maintains |
|------|-----------|-------------------|
| Template compatibility test | Write `test/template_compatibility_test.go` | Add entries when new templates are created |
| SDK matrix workflow | Write `.github/workflows/sdk-version-matrix.yml` | Rotate `SLACK_WEBHOOK_URL`; optionally add cross-repo triggers |
| PTY test wrapper | Write `test/pty_helper_test.go` + wizard tests | Add tests when wizard prompts change |
| macOS runner | Add to CI matrix | No maintenance needed |
| QA runner skill | Write `.cursor/skills/cre-qa-runner/SKILL.md` | Update when runbook or template list changes |
| Add-template skill | Write `.cursor/skills/cre-add-template/SKILL.md` | Update when the template creation process changes |
| Documentation | This document + analysis docs in `docs/testing-framework/` | Keep current as architecture evolves |

**Maintenance effort estimates**:
- New template added: ~1 hour (create files, update registry, add test entry, run tests)
- SDK version bump: ~15 minutes (update pin in `go_module_init.go` or `package.json.tpl`, verify CI passes)
- Runbook change: ~10 minutes (update skill if test steps changed)
- Credential rotation: ~5 minutes (update GitHub Secrets)

---

## 7. Timeline

### Week 1: CI Safety Net

| Day | Deliverable | Output |
|-----|-------------|--------|
| 1 | Template compatibility test file | `test/template_compatibility_test.go` passing locally for all 5 templates |
| 2 | Template compatibility CI job | `ci-test-template-compat` job in `pull-request-main.yml`, green on PR |
| 3-4 | SDK version matrix workflow | `.github/workflows/sdk-version-matrix.yml`, first nightly run passes |
| 4 | macOS CI runner | Added to matrix, first green run |

### Week 2: Coverage + AI + Handoff

| Day | Deliverable | Output |
|-----|-------------|--------|
| 5-6 | PTY test wrapper + wizard tests | `test/pty_helper_test.go`, `test/wizard_test.go` covering wizard flow, cancel, validation |
| 7 | AI QA runner skill | `.cursor/skills/cre-qa-runner/SKILL.md`, first successful AI-generated report |
| 7 | Add-template skill | `.cursor/skills/cre-add-template/SKILL.md` |
| 8 | Documentation + handoff session | Updated docs, walkthrough with CRE team |

**Total: ~8 working days across 2 weeks.**

---

## 8. Appendix

### Detailed Analysis Documents

These documents in `docs/testing-framework/` contain the deep-dive analysis that informed this plan:

| Document | Contents |
|----------|----------|
| [01-testing-framework-architecture.md](01-testing-framework-architecture.md) | Three-tier framework design, component diagrams, failure detection matrix |
| [02-test-classification-matrix.md](02-test-classification-matrix.md) | All 117 runbook tests classified as Script/AI/Manual with rationale |
| [03-poc-specification.md](03-poc-specification.md) | PoC spec for template validation (two-track design, agent prompt, report format) |
| [04-ci-cd-integration-design.md](04-ci-cd-integration-design.md) | CI/CD workflow designs, cross-repo triggers, cost analysis |

### Evidence

- [Windows QA Report (2026-02-12)](../../2026-02-12%20QA%20Test%20Report%20-%20CRE%20CLI%20-%20windows.md): Claude Code executed the full runbook against the preview binary. 75 PASS, 1 FAIL, 18 SKIP (TTY-dependent), 9 N/A. Demonstrates AI can run the runbook and produce a structured report.
- [QA Developer Runbook](../../.qa-developer-runbook.md): The 103-test manual testing guide that defines the complete validation scope.
