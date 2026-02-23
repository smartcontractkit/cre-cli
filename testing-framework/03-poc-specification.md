# PoC Specification: AI-Driven Template Validation

> A detailed specification for a proof-of-concept that demonstrates AI-augmented testing of CRE CLI templates. This PoC validates templates (init + build + simulate) across all 5 template types and supports both embedded baseline mode and branch-gated dynamic template pull mode.

---

## Table of Contents

1. [PoC Scope and Goals](#1-poc-scope-and-goals)
2. [Architecture](#2-architecture)
3. [Component Specifications](#3-component-specifications)
4. [Test Scenarios](#4-test-scenarios)
5. [AI Agent Prompt Design](#5-ai-agent-prompt-design)
6. [Report Format](#6-report-format)
7. [Implementation Phases](#7-implementation-phases)
8. [Success Criteria](#8-success-criteria)
9. [Known Constraints](#9-known-constraints)

---

## 1. PoC Scope and Goals

### 1.0 Template Source Mode Scope

- Embedded mode: required baseline for PoC execution now.
- Dynamic pull mode: second PoC track that is branch-gated until external branch/repo links are provided.
- Every PoC run must record the template source mode used.

### 1.1 In Scope

- **Template compatibility testing**: All 5 templates (Go HelloWorld, Go PoR, TS HelloWorld, TS PoR, TS ConfHTTP)
- **Template-source provenance capture**: For dynamic mode, capture repo/ref/commit in evidence.
- **Deterministic script layer**: Go test that exercises init + build + simulate for every template
- **AI agent layer**: Claude Code agent that runs the full template validation, interprets results, and generates a structured report
- **Single platform**: macOS or Linux (not multi-platform for PoC)
- **Single environment**: PRODUCTION or STAGING
- **Report generation**: Structured markdown report matching `.qa-test-report-template.md` format

### 1.2 Out of Scope (for PoC)

- Deploy/pause/activate/delete lifecycle (requires on-chain TX and real credentials)
- Secrets management
- Account key management
- Interactive wizard testing (PTY interaction)
- Browser-based auth flows
- Multi-platform matrix
- CI/CD integration (covered in separate design doc)
- SDK version matrix testing (covered in CI/CD design)

### 1.3 Goals

1. **Demonstrate** that all 5 templates can be automatically validated with init + build + simulate
2. **Prove** that an AI agent can run the validation and produce a human-readable test report
3. **Identify** the boundary between what scripts can validate and where AI adds value
4. **Establish** the test report format and quality bar for ongoing use
5. **Measure** execution time and cost for template testing

---

## 2. Architecture

### 2.1 Two-Track Design

The PoC has two parallel tracks that validate the same thing differently:

```
Track A: Deterministic Script (Go Test)
  - test/template_compatibility_test.go
  - Data-driven: iterates over all template IDs
  - Binary pass/fail: exit code 0 or not
  - Runs in CI alongside existing E2E tests
  - No AI involvement

Track B: AI Agent (Claude Code)
  - Reads .qa-developer-runbook.md sections 5-7 as spec
  - Executes init + build + simulate for each template
  - Interprets output semantically
  - Generates .qa-test-report-YYYY-MM-DD.md
  - Runs on-demand or pre-release

Template source validation overlays both tracks:

- Embedded baseline track: validates current runtime behavior.
- Dynamic pull track (branch-gated): validates fetch + init/build/simulate parity and records source provenance.
```

Track A is the foundation -- it catches breakage in CI. Track B adds interpretive depth and report generation. The PoC builds both to demonstrate their respective strengths.

### 2.2 Component Diagram

```
+----------------------------------------------------------------+
|                                                                  |
|  TRACK A: Deterministic Script                                  |
|                                                                  |
|  test/template_compatibility_test.go                            |
|    |                                                             |
|    +-- TestMain: build CLI binary (existing)                    |
|    +-- TestTemplateCompatibility_GoHelloWorld (template 2)      |
|    +-- TestTemplateCompatibility_GoPoR (template 1)             |
|    +-- TestTemplateCompatibility_TSHelloWorld (template 3)      |
|    +-- TestTemplateCompatibility_TSPoR (template 4)             |
|    +-- TestTemplateCompatibility_TSConfHTTP (template 5)        |
|    |                                                             |
|    Each sub-test:                                               |
|      1. cre init -p <name> -t <id> -w <wf> [--rpc-url <url>]  |
|      2. Verify file structure                                   |
|      3. Build (go build / bun install)                          |
|      4. Simulate (cre workflow simulate --non-interactive)      |
|      5. Assert exit codes and key output strings                |
|                                                                  |
+----------------------------------------------------------------+

+----------------------------------------------------------------+
|                                                                  |
|  TRACK B: AI Agent                                              |
|                                                                  |
|  Entrypoint: CLAUDE.md (agent instructions)                     |
|    |                                                             |
|    +-- Read .qa-developer-runbook.md (sections 5-7)             |
|    +-- For each template (1-5):                                 |
|    |     +-- Run cre init with appropriate flags                |
|    |     +-- Verify project structure                           |
|    |     +-- Run build step                                     |
|    |     +-- Run simulate                                       |
|    |     +-- Interpret output (semantic validation)             |
|    |     +-- Record results in report                           |
|    +-- Generate .qa-test-report-YYYY-MM-DD.md                   |
|    +-- Print summary                                            |
|                                                                  |
+----------------------------------------------------------------+
```

---

## 3. Component Specifications

### 3.1 Track A: Deterministic Script

#### File: `test/template_compatibility_test.go`

**Design**: Data-driven test that iterates over a template registry, exercising init + build + simulate for each.

**Template registry** (should mirror `cmd/creinit/creinit.go:languageTemplates`):

```
Template ID | Language   | Name        | Build Command           | Extra Init Flags
------------|------------|-------------|-------------------------|------------------
1           | Go         | Go PoR      | go build ./...          | --rpc-url <default>
2           | Go         | Go Hello    | go build ./...          |
3           | TypeScript | TS Hello    | bun install             |
4           | TypeScript | TS PoR      | bun install             | --rpc-url <default>
5           | TypeScript | TS ConfHTTP | bun install             |
```

**Test flow per template**:

```
1. Create temp directory
2. Run: cre init -p test-<id> -t <id> -w test-wf [--rpc-url ...] --project-root <tempdir>
3. Assert: exit code 0
4. Assert: expected files exist:
   - project.yaml
   - .env
   - test-wf/workflow.yaml
   - test-wf/main.go (Go) or test-wf/main.ts (TS)
   - For Go PoR: test-wf/workflow.go, test-wf/workflow_test.go, contracts/
   - For TS: test-wf/package.json, test-wf/tsconfig.json

5. For Go templates:
   a. Run: go build ./... (in project root)
   b. Assert: exit code 0

6. For TS templates:
   a. Run: bun install (in workflow dir)
   b. Assert: exit code 0

7. Set up mock GraphQL server (for auth -- same pattern as existing E2E tests)
8. Set CRE_CLI_GRAPHQL_URL to mock URL
9. Set CRE_API_KEY=test-api
10. Set CRE_ETH_PRIVATE_KEY=<test key>

11. Run: cre workflow simulate test-wf --non-interactive --trigger-index=0
        --project-root <tempdir> --cli-env-file <env>
12. Assert: exit code 0
13. Assert: output contains "Workflow compiled" or language-equivalent marker
14. Assert: output contains workflow result (template-specific)

15. Clean up temp directory
```

**Expected assertions per template**:

| Template | Init Files | Build Check | Simulate Output |
|----------|-----------|-------------|-----------------|
| Go HelloWorld (2) | main.go, workflow.yaml, project.yaml, .env | `go build ./...` exit 0 | Contains "Fired at" or timestamp |
| Go PoR (1) | main.go, workflow.go, workflow_test.go, contracts/ | `go build ./...` exit 0 | Contains PoR data or "Write report" |
| TS HelloWorld (3) | main.ts, package.json, tsconfig.json | `bun install` exit 0 | Contains "Hello world!" |
| TS PoR (4) | main.ts, package.json, contracts/abi/ | `bun install` exit 0 | Contains PoR data or numeric result |
| TS ConfHTTP (5) | main.ts, package.json | `bun install` exit 0 | Contains HTTP response or result |

**Mock server requirements**:

The simulate command requires auth (it calls `AttachCredentials`). The existing E2E tests solve this with a mock GraphQL server that handles `getOrganization`. The template compatibility tests should reuse this pattern:

```
Mock GraphQL server:
  POST /graphql:
    if body contains "getOrganization":
      return {"data":{"getOrganization":{"organizationId":"test-org-id"}}}
    else:
      return 400
```

This is identical to the pattern in `test/init_and_simulate_ts_test.go`.

#### File changes required for auto-discovery

Currently, template IDs are defined in `cmd/creinit/creinit.go` as unexported variables. For the test to auto-discover templates, one of these approaches is needed:

**Option A: Export the template registry**
- Change `languageTemplates` to `LanguageTemplates` in `cmd/creinit/creinit.go`
- Test imports and iterates over the exported slice

**Option B: Duplicate the template list in the test**
- Maintain a parallel list in the test file
- Risk: list goes out of sync when new templates are added
- Mitigation: add a test that verifies the test list matches the code list (by count or by importing the package)

**Option C: Data-driven via test table**
- Define templates in a Go test table with all metadata needed
- Add a "canary" test that compares the count against the actual `languageTemplates` length

Option A is cleanest. Option C is most pragmatic for a PoC that doesn't modify production code.

### 3.2 Track B: AI Agent

#### Agent Instructions File: `CLAUDE.md` (or equivalent)

The AI agent needs a structured prompt that tells it exactly what to do. This should be a markdown file that serves as the agent's instructions when invoked.

**Required sections in agent instructions**:

1. **Context**: You are testing the CRE CLI's template compatibility. The CLI scaffolds projects from embedded templates.
2. **Prerequisites**: Check that `go`, `bun`, `node`, and `cre` binary are available. Report versions.
3. **Test Matrix**: For each template ID (1-5), perform init + build + simulate.
4. **Execution Steps**: Detailed steps for each template (mirror the runbook).
5. **Validation Criteria**: What constitutes PASS vs FAIL for each step.
6. **Output Format**: Generate a report matching `.qa-test-report-template.md`.
7. **Error Handling**: If a step fails, record the failure, capture output, and continue to the next template.

#### Agent Environment Requirements

```
Required:
  - CRE CLI binary (pre-built or buildable from source)
  - Go 1.25.5+
  - Bun 1.2.21+
  - Node.js 20.13.1+
  - CRE_API_KEY (for simulate auth)
  - Network access (for go get, bun install, npm registry)
  - Writable temp directory

Optional:
  - Foundry/Anvil (only needed for EVM-trigger simulation)
  - CRE_ETH_PRIVATE_KEY (only needed for broadcast simulation)
```

#### Agent Decision Points

These are the moments where the AI agent provides value beyond a script:

| Decision Point | Script Behavior | AI Agent Behavior |
|---------------|----------------|-------------------|
| `go build` fails with import error | Report: "exit code 1" | Read error: "package X not found in cre-sdk-go v1.2.0"; diagnose: "SDK version may have removed this package" |
| `bun install` fails with resolution error | Report: "exit code 1" | Read error: "could not resolve @chainlink/cre-sdk@^1.0.9"; diagnose: "npm registry may be down or version range no longer satisfiable" |
| Simulate output is empty | Report: "output does not contain expected string" | Analyze: check if WASM compiled, check if trigger fired, check engine logs for error |
| PoR simulate returns unexpected value | Report: "PASS" (exit code 0) | Validate: "returned value is 0.0 which is likely wrong for a PoR feed"; flag as potential issue |
| New template added (ID 6) | Not tested (not in test table) | Can be instructed: "test all templates listed in `cre init --help`" |

---

## 4. Test Scenarios

### 4.1 Template Compatibility Scenarios

For each template, the PoC validates three layers:

```
Layer 1: Scaffolding (cre init)
  - Does the CLI create the expected directory structure?
  - Are all required files present?
  - Is project.yaml well-formed?
  - Is workflow.yaml well-formed?
  - For Go: is go.mod created with correct module name and SDK version?
  - For TS: is package.json present with correct dependencies?
  - For PoR: is the RPC URL in project.yaml?
  - For PoR Go: are contracts/ present?

Layer 2: Build
  - For Go: does `go build ./...` succeed?
  - For TS: does `bun install` succeed?
  - Are there any warnings that indicate future breakage?

Layer 3: Simulate
  - Does the workflow compile to WASM?
  - Does the simulation engine start?
  - Does the trigger fire?
  - Does the workflow produce output?
  - Is the output semantically valid?
```

### 4.2 Specific Test Cases per Template

#### Template 1: Go PoR

```
Init:
  Command: cre init -p test-go-por -t 1 -w por-wf --rpc-url https://ethereum-sepolia-rpc.publicnode.com
  Expected files:
    - test-go-por/go.mod (module: test-go-por)
    - test-go-por/por-wf/main.go
    - test-go-por/por-wf/workflow.go
    - test-go-por/por-wf/workflow_test.go
    - test-go-por/contracts/evm/src/abi/ (ABI files)
    - test-go-por/contracts/evm/src/generated/ (Go bindings)
    - test-go-por/secrets.yaml
    - test-go-por/project.yaml (contains RPC URL)
    - test-go-por/.env

Build:
  Command: go build ./... (in test-go-por/)
  Expected: exit code 0, no errors

Go Tests:
  Command: go test ./... (in test-go-por/por-wf/)
  Expected: exit code 0, all tests pass
  Note: This runs the workflow_test.go that comes with the template

Simulate:
  Command: cre workflow simulate por-wf --non-interactive --trigger-index=0
  Expected:
    - "Workflow compiled" in output
    - Simulation produces result with PoR data
    - Exit code 0
  AI validation:
    - Output should contain a numeric value (the PoR result)
    - Value should be positive and non-zero
    - "Write report succeeded" suggests the EVM write action completed
```

#### Template 2: Go HelloWorld

```
Init:
  Command: cre init -p test-go-hello -t 2 -w hello-wf
  Expected files:
    - test-go-hello/go.mod
    - test-go-hello/hello-wf/main.go
    - test-go-hello/hello-wf/workflow.yaml
    - test-go-hello/project.yaml
    - test-go-hello/.env

Build:
  Command: go build ./... (in test-go-hello/)
  Expected: exit code 0

Simulate:
  Command: cre workflow simulate hello-wf --non-interactive --trigger-index=0
  Expected:
    - "Workflow compiled" in output
    - Result contains "Fired at" or a timestamp
    - Exit code 0
  AI validation:
    - Result should be JSON with a "Result" key
    - Timestamp should be recent (within last few minutes)
```

#### Template 3: TS HelloWorld

```
Init:
  Command: cre init -p test-ts-hello -t 3 -w hello-wf
  Expected files:
    - test-ts-hello/hello-wf/main.ts
    - test-ts-hello/hello-wf/package.json
    - test-ts-hello/hello-wf/tsconfig.json
    - test-ts-hello/hello-wf/workflow.yaml
    - test-ts-hello/project.yaml
    - test-ts-hello/.env

Install:
  Command: bun install (in test-ts-hello/hello-wf/)
  Expected: exit code 0, node_modules/ created

Simulate:
  Command: cre workflow simulate hello-wf --non-interactive --trigger-index=0
  Expected:
    - Output contains "Hello world!"
    - Exit code 0
  AI validation:
    - Simple string output; any variation of "Hello world" is acceptable
```

#### Template 4: TS PoR

```
Init:
  Command: cre init -p test-ts-por -t 4 -w por-wf --rpc-url https://ethereum-sepolia-rpc.publicnode.com
  Expected files:
    - test-ts-por/por-wf/main.ts
    - test-ts-por/por-wf/package.json
    - test-ts-por/por-wf/tsconfig.json
    - test-ts-por/por-wf/workflow.yaml
    - test-ts-por/contracts/abi/*.ts (ABI files)
    - test-ts-por/project.yaml (contains RPC URL)
    - test-ts-por/.env

Install:
  Command: bun install (in test-ts-por/por-wf/)
  Expected: exit code 0

Simulate:
  Command: cre workflow simulate por-wf --non-interactive --trigger-index=0
  Expected:
    - Output contains PoR result
    - Exit code 0
  AI validation:
    - Result should contain a numeric value
    - Value should be positive (represents financial data)
```

#### Template 5: TS ConfHTTP (Hidden)

```
Init:
  Command: cre init -p test-ts-conf -t 5 -w conf-wf
  Expected files:
    - test-ts-conf/conf-wf/main.ts
    - test-ts-conf/conf-wf/package.json
    - test-ts-conf/conf-wf/tsconfig.json
    - test-ts-conf/conf-wf/workflow.yaml
    - test-ts-conf/project.yaml
    - test-ts-conf/.env

Install:
  Command: bun install (in test-ts-conf/conf-wf/)
  Expected: exit code 0

Simulate:
  Command: cre workflow simulate conf-wf --non-interactive --trigger-index=0
  Expected:
    - Exit code 0 (or documented expected behavior for confidential HTTP without secrets)
  AI validation:
    - This template uses confidential HTTP; simulation may require secrets
    - AI should document the behavior and any prerequisites
  Note: This is a hidden template; behavior should still be validated
```

---

## 5. AI Agent Prompt Design

### 5.1 System Prompt Structure

The AI agent prompt should be structured as a markdown document with clear sections:

```markdown
# CRE CLI Template Validation Agent

## Your Role
You are a QA engineer testing the CRE CLI's template system. Your job is to validate
that every template the CLI ships can be successfully initialized, built, and simulated.

## Prerequisites
Before starting, verify these tools are available:
- cre (the CLI binary) -- run `cre version`
- go -- run `go version` (need 1.25.5+)
- bun -- run `bun --version` (need 1.2.21+)
- node -- run `node --version` (need 20.13.1+)

Record all version numbers in the report.

## Test Matrix
Test every template the CLI offers. Get the list by running:
  cre init --help
and identifying all template IDs mentioned.

Currently known templates:
  ID 1: Go PoR
  ID 2: Go HelloWorld
  ID 3: TS HelloWorld
  ID 4: TS PoR
  ID 5: TS ConfHTTP (hidden, but still testable with -t 5)

## For Each Template
### Step 1: Init
  mkdir -p /tmp/cre-template-test && cd /tmp/cre-template-test
  cre init -p test-tpl-<ID> -t <ID> -w test-wf [--rpc-url <url> if PoR]

  Verify:
  - Exit code 0
  - Expected files exist (see template-specific expectations)
  - project.yaml is valid YAML
  - workflow.yaml is valid YAML

### Step 2: Build
  For Go: cd test-tpl-<ID> && go build ./...
  For TS: cd test-tpl-<ID>/test-wf && bun install

  Verify:
  - Exit code 0
  - No error output
  - If warnings appear, record them

### Step 3: Simulate
  Set up environment:
    export CRE_API_KEY=<provided>
    export CRE_ETH_PRIVATE_KEY=<test-key>

  Run:
    cre workflow simulate test-wf --non-interactive --trigger-index=0

  Verify:
  - Exit code 0
  - Output contains workflow result
  - Result is semantically valid (not empty, not error)

### Step 4: Record
  For each step, record:
  - Status: PASS / FAIL / SKIP
  - Command executed
  - Relevant output (truncated if long)
  - For FAIL: what happened vs. what was expected

## Error Handling
- If a template fails to init, mark it FAIL and continue to the next template
- If build fails, still try simulate (it may give a different/better error)
- If simulate fails, capture the full error output for diagnosis
- Never abort the entire test run because of one template failure

## Output Format
Generate a report matching this structure:
[follows .qa-test-report-template.md format]
```

### 5.2 Key Prompt Engineering Decisions

**Why structured markdown over free-form instructions**:
- AI agents follow structured, enumerated steps more reliably
- Explicit verification criteria reduce hallucinated PASSes
- "For each template" pattern ensures completeness

**Why explicit error handling instructions**:
- Without them, AI agents tend to stop at the first failure
- "Continue to next template" is critical for getting a complete report
- "Still try simulate even if build fails" catches cases where the build step is optional

**Why version checks first**:
- Tool version mismatches are the most common cause of false failures
- Recording versions in the report enables debugging without reproducing

### 5.3 Context Documents to Provide

The AI agent should have access to (read-only):

1. `.qa-developer-runbook.md` -- the human test spec (for reference)
2. `.qa-test-report-template.md` -- the report format to follow
3. `cmd/creinit/creinit.go` -- template registry (to verify completeness)
4. `cmd/creinit/go_module_init.go` -- SDK version pins (for diagnosis)
5. Template source files (`cmd/creinit/template/workflow/`) -- to understand expected behavior

---

## 6. Report Format

The AI agent should produce a report that matches the existing `.qa-test-report-template.md` format. Here is the template for the sections relevant to the PoC:

```markdown
# QA Test Report -- CRE CLI Template Compatibility

## Run Metadata

| Field | Value |
|-------|-------|
| Date | YYYY-MM-DD |
| Tester | Claude Code (automated) |
| Test Mode | Template Compatibility |
| Binary Source | [build from source / pre-built binary path] |
| Branch | [branch name] |
| Commit | [git rev-parse HEAD] |
| OS | [uname -a or equivalent] |
| Go Version | [go version output] |
| Bun Version | [bun --version output] |
| Node Version | [node --version output] |
| CRE CLI Version | [cre version output] |

## Template Test Results

### Template 1: Go PoR

| Step | Status | Notes |
|------|--------|-------|
| Init | PASS/FAIL | [notes] |
| File structure | PASS/FAIL | [missing files if any] |
| Build (go build) | PASS/FAIL | [error output if any] |
| Go tests | PASS/FAIL | [test output summary] |
| Simulate | PASS/FAIL | [result summary] |

Evidence:
[key command output, truncated]

### Template 2: Go HelloWorld
[same structure]

### Template 3: TS HelloWorld
[same structure]

### Template 4: TS PoR
[same structure]

### Template 5: TS ConfHTTP
[same structure]

## Summary

| Template | Init | Build | Simulate | Overall |
|----------|------|-------|----------|---------|
| Go PoR (1) | PASS | PASS | PASS | PASS |
| Go Hello (2) | PASS | PASS | PASS | PASS |
| TS Hello (3) | PASS | PASS | PASS | PASS |
| TS PoR (4) | PASS | PASS | PASS | PASS |
| TS ConfHTTP (5) | PASS | PASS | SKIP | PASS* |

## Issues Found
[list any failures, unexpected behaviors, or warnings]

## Recommendations
[any suggestions based on observations]

## Execution Time
| Template | Init | Build | Simulate | Total |
|----------|------|-------|----------|-------|
[timing data]

Total execution time: X minutes
```

---

## 7. Implementation Phases

### Phase 1: Script Layer (Track A)

**Deliverable**: `test/template_compatibility_test.go`

**Steps**:
1. Create test file with template test table
2. Implement `TestTemplateCompatibility` that iterates over all templates
3. Reuse existing mock GraphQL pattern from `test/init_and_simulate_ts_test.go`
4. Add to CI pipeline in `pull-request-main.yml`
5. Verify all 5 templates pass

**Dependencies**:
- Existing E2E test infrastructure (`test/common.go`, `test/cli_test.go`)
- Mock GraphQL server pattern
- CI runners with Go, Bun, Node

**Estimated effort**: 2-3 days

### Phase 2: AI Agent Instructions

**Deliverable**: Agent instruction document (CLAUDE.md or similar)

**Steps**:
1. Write structured agent prompt (see Section 5)
2. Define verification criteria for each template
3. Define report format
4. Test with a manual Claude Code invocation
5. Iterate on prompt based on agent behavior

**Dependencies**:
- Claude Code CLI or API access
- CRE_API_KEY for test environment
- Pre-built CLI binary

**Estimated effort**: 1-2 days

### Phase 3: Validation and Comparison

**Steps**:
1. Run Track A (script) against current CLI binary
2. Run Track B (AI agent) against same binary
3. Compare results:
   - Do both tracks agree on PASS/FAIL?
   - Where does the AI agent provide additional insight?
   - Where is the AI agent wrong or misleading?
4. Document the delta between script and AI capabilities
5. Calibrate AI prompt based on findings

**Estimated effort**: 1 day

### Phase 4: Documentation and Handoff

**Steps**:
1. Document how to run both tracks
2. Document how to add new templates to the test
3. Document how to read and interpret AI agent reports
4. Create runbook for the CRE team to maintain the tests

**Estimated effort**: 1 day

**Total PoC estimate**: 5-7 days

---

## 8. Success Criteria

### 8.1 Hard Requirements

- [ ] All 5 templates validated (init + build + simulate) by Track A (script)
- [ ] Track A test runs in CI and catches a deliberately broken template
- [ ] AI agent (Track B) successfully produces a structured test report for all 5 templates
- [ ] Track B report is actionable: a human reader can determine pass/fail and understand failures
- [ ] Both tracks agree on pass/fail for all 5 templates

### 8.2 Soft Requirements

- [ ] AI agent provides diagnostic insight that the script does not (e.g., "this failure is likely caused by SDK version X removing package Y")
- [ ] Total Track A execution time is under 10 minutes
- [ ] Total Track B execution time is under 30 minutes
- [ ] Report format is compatible with existing `.qa-test-report-template.md`

### 8.3 Non-Requirements (Do NOT Optimize For)

- Multi-platform support (PoC runs on one platform)
- Deploy lifecycle testing (out of scope)
- SDK version matrix (out of scope for PoC; covered in CI/CD design)
- Cost optimization (PoC is proof-of-concept, not production)

---

## 9. Known Constraints

### 9.1 Template 5 (TS ConfHTTP) May Require Special Handling

This template is marked `Hidden: true` in the template registry. It uses confidential HTTP, which requires secrets to be configured. In simulation:
- The `DirectConfidentialHTTPAction` capability is used
- It needs `secretsPath` from `workflow.yaml`
- Without actual secrets, the simulation may produce an error or empty result

The PoC should document whatever behavior occurs rather than marking it as FAIL. If the template requires secrets to simulate, this is a "SKIP with reason" rather than a failure.

### 9.2 Go PoR Template Simulation May Hit External APIs

The Go PoR template's `config.json` contains a URL for fetching proof-of-reserve data. During simulation, this URL is hit by the HTTP capability. If the external API is down or rate-limited, simulation may fail for reasons unrelated to the template.

Mitigation options:
- Accept occasional flakiness and document it
- Provide a mock URL override mechanism (like `test/multi_command_flows/workflow_simulator_path.go` does)
- The existing E2E test (`test/init_and_binding_generation_and_simulate_go_test.go`) also hits real URLs during simulation

### 9.3 TS Templates Depend on npm Registry Availability

`bun install` resolves `@chainlink/cre-sdk@^1.0.9` from the npm registry. If the registry is down or the package version is yanked, the build step fails.

This is actually a feature for Tier 1 tests: it validates that the dependency range in `package.json.tpl` is still satisfiable. But it means tests can fail due to external registry issues.

### 9.4 Go Templates Depend on Go Module Proxy

`go get cre-sdk-go@v1.2.0` resolves from the Go module proxy. The same registry-availability concern applies.

### 9.5 Simulation Requires Anvil for EVM-Trigger Templates

Templates with EVM triggers need Anvil running to simulate. The Go PoR and TS PoR templates use cron triggers but also make EVM calls (read balances, write reports). The simulation engine creates EVM clients from project.yaml RPCs.

For the PoC, ensure Anvil is available (it is already a CI dependency) or accept that EVM-dependent simulations may behave differently without it. The existing E2E tests handle this with `StartAnvil()` and pre-baked state.
