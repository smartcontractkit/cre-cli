# Runbook Phase Map

Use this phase order when executing `.qa-developer-runbook.md`.

## Phase 0: Preflight

- Verify toolchain versions and env status.
- Initialize report copy from `.qa-test-report-template.md`.
- Populate Run Metadata before tests.
- Determine template source mode for this run: embedded baseline or branch-gated dynamic pull.

Evidence required:
- `go version`, `node --version`, `bun --version`, `anvil --version`.
- `./cre version`.
- Set/unset status for `CRE_API_KEY`, `ETH_PRIVATE_KEY`, `CRE_ETH_PRIVATE_KEY`, `CRE_CLI_ENV`.
- Template source metadata: mode, and when dynamic mode is active, template repo/ref/commit.

## Phase 1: Build and Baseline

Runbook sections:
- 2. Build and Smoke Test
- 3. Unit and E2E Test Suite

Evidence required:
- `make build`, smoke command outputs.
- `make lint`, `make test`, `make test-e2e` summaries.

## Phase 2: Auth and Init

Runbook sections:
- 4. Account Creation and Authentication
- 5. Project Initialization
- 15. Wizard UX Verification (non-visual portions first)

Evidence required:
- Command output and explicit status for login/logout/whoami/api key/auth-gated prompt.
- Init wizard and non-interactive flow outputs.

## Phase 3: Template and Simulate

Runbook sections:
- 6. Template Validation - Go
- 7. Template Validation - TypeScript
- 8. Workflow Simulate

Evidence required:
- Init/build/install/simulate results for each template under test.
- Non-interactive trigger cases and error cases.

## Phase 4: Lifecycle and Data Plane

Runbook sections:
- 9. Deploy/Pause/Activate/Delete
- 10. Account Key Management
- 11. Secrets Management
- 13. Environment Switching

Evidence required:
- Per-command status and transaction/result evidence.
- Secret operation evidence must include names only, never values.

## Phase 5: Utilities and Negatives

Runbook sections:
- 12. Utility Commands
- 14. Edge Cases and Negative Tests

Evidence required:
- Version/update/bindings/completion outcomes.
- Negative case expected-vs-actual notes.

## Phase 6: Closeout

- Fill checklist summary and final verdict.
- Confirm PASS/FAIL/SKIP/BLOCKED totals align with section statuses.
