# Validation Execution Strategy

*How to execute `validation-and-report-plan.md` using parallel subagents in Cursor.*

---

## Dependency Graph

```
Wave 0: Build + Environment
   │
   ├──────────────────────────────────────────┐
   │              Wave 1 (parallel)           │
   │                                          │
   ├─► Agent A: Merge Gates (Stream 1)        │
   ├─► Agent B: Skills & Scripts (Stream 3)   │
   ├─► Agent C: CI/CD + Playwright (S2 + S4)  │
   └─► Agent D: Submodules + Docs (Stream 7)  │
                                              │
   ┌──────────────────────────────────────────┘
   │
   ▼
Wave 2: QA Report + Evidence Contract (Streams 5 + 6)
   │
   ▼
Wave 3: Gap Register + Final Report (Phase 9)
```

---

## Wave 0: Build & Environment Setup

**Mode:** Sequential, single operator or single agent.
**Blocks:** Everything in Waves 1-3.
**Time:** ~5-10 min

### Steps

```bash
# Build the CLI
make build && ./cre version && ./cre --help

# Clone external template workspace
make setup-submodules

# Verify all required tools are installed
command -v go expect bun node forge anvil
go version && bun --version && node -v && forge --version && anvil --version
```

### Done When

- `./cre version` prints output
- `cre-templates/` directory exists
- All tool checks pass (or gaps are documented)

---

## Wave 1: Parallel Validation (4 Agents)

**Prerequisite:** Wave 0 complete.
**Max concurrency:** 4 agents (Cursor limit).
**Time:** ~30 min wall-clock (longest agent determines duration).

### Agent A: Merge Gates (Stream 1)

**Scope:** Template compatibility, drift canary, path filter, E2E.
**Parallel-safe:** Yes -- Go tests use `t.TempDir()` for isolation.
**Est. runtime:** ~20 min (template compat has 20-min timeout)

**Prompt outline:**
1. Run `go test -v -timeout 20m -run TestTemplateCompatibility ./test/` and capture per-template results.
2. Verify Template 5 uses `simulateMode: "compile-only"` and behaves as designed.
3. Run `make test-e2e` and confirm existing E2E tests still pass.
4. Inspect `test/template_compatibility_test.go` for drift canary logic -- describe how it detects template/table mismatch.
5. Inspect `.github/workflows/pull-request-main.yml` lines 16-86 for path filter logic and `merge_group` handling.
6. Check if the path filter could miss `internal/` changes that affect template scaffolding.
7. Report: per-template PASS/FAIL, canary mechanism description, path filter analysis, E2E results, gaps found.

**Evidence to capture:**
- Per-template test output (PASS/FAIL + runtime)
- Drift canary mechanism description
- Path filter coverage analysis
- E2E test results
- Any gaps or failures

### Agent B: Skills & Scripts (Stream 3)

**Scope:** All 7 scripts + 2 expect scripts + symlink + audit report.
**Parallel-safe:** Yes, but use a temp working directory for expect scripts to avoid collision with Agent A.
**Est. runtime:** ~30 min

**Prompt outline:**
1. Run each script and capture exit code + output:
   - `.claude/skills/cre-qa-runner/scripts/env_status.sh`
   - `.claude/skills/cre-qa-runner/scripts/collect_versions.sh`
   - `.claude/skills/cre-qa-runner/scripts/init_report.sh`
   - `.claude/skills/cre-add-template/scripts/template_gap_check.sh`
   - `.claude/skills/cre-add-template/scripts/print_next_steps.sh`
2. Authenticate via Playwright browser auth (requires `CRE_USER_NAME` and `CRE_PASSWORD` in `.env`), then run expect scripts from repo root:
   - `expect .claude/skills/cre-cli-tui-testing/tui_test/pty-smoke.expect`
   - `expect .claude/skills/cre-cli-tui-testing/tui_test/pty-overwrite.expect`
3. Check symlink: `ls -la .claude/skills/using-cre-cli/references/` -- does `@docs` resolve?
4. Read `.claude/skills/skill-audit-report.md` -- is it current and valid?
5. Verify no raw secrets appear in any script output.
6. Report: per-script result table (script | exit code | output summary | issues), expect script results with timing notes, symlink status, audit report status.

**Evidence to capture:**
- Per-script exit codes and output summaries
- Expect script results (exit 0 or failure details + timing)
- Symlink resolution status
- Skill audit report validity
- Missing tool issues
- Secret hygiene confirmation

### Agent C: CI/CD Audit + Playwright Status (Streams 2 + 4)

**Scope:** Read-only inspection of workflow YAML, branch protection, and Playwright status.
**Parallel-safe:** Yes -- entirely read-only.
**Est. runtime:** ~15 min

**Prompt outline:**
1. Read `.github/workflows/pull-request-main.yml` and document:
   - All jobs and their trigger conditions
   - `ci-test-template-compat` matrix (which OSes?)
   - Artifact retention configuration
   - Whether this is set as a required check (inspect for any branch protection hints)
2. Check for existence of:
   - `.github/workflows/sdk-version-matrix.yml` (expected: does not exist)
   - `.github/workflows/ai-validation.yml` (expected: does not exist)
3. Read `testing-framework/04-ci-cd-integration-design.md` and assess: are the nightly/AI workflow designs complete enough for someone to implement directly?
4. Check Playwright status:
   - `.claude/skills/playwright-cli/SKILL.md` (expected: does not exist)
   - Any Playwright config files, test files, or scripts in the repo
   - References in `AGENTS.md` (lines 107, 122)
   - References in `.claude/skills/cre-cli-tui-testing/references/setup.md`
5. Report: CI/CD configuration summary, design-only gap list, Playwright status classification, implementation readiness assessment for nightly workflow.

**Evidence to capture:**
- Workflow job list with trigger conditions
- Matrix configuration
- Design-only components list (with design doc references)
- Playwright existence check results
- Implementation readiness assessment

### Agent D: Submodules + Documentation Accuracy (Stream 7)

**Scope:** Workspace lifecycle, AGENTS.md audit, cross-commit consistency, testing framework docs.
**Parallel-safe:** Yes -- operates on `cre-templates/` dir and reads doc files.
**Est. runtime:** ~30 min

**Prompt outline:**
1. Test submodules lifecycle:
   - `make setup-submodules` (should create `cre-templates/`)
   - Check `.gitignore` for managed section
   - `make update-submodules` (should update without errors)
   - `make clean-submodules` (should remove `cre-templates/`)
2. Verify `yq` dependency -- what happens without it?
3. Audit `AGENTS.md`:
   - Compare every skill in the Skill Map section to actual files under `.claude/skills/`
   - Flag any referenced skills that don't exist (expected: `playwright-cli`)
   - Verify the Component Map paths resolve to real directories
   - Verify "Template Source Modes" section accurately describes embedded vs. dynamic
4. Cross-check testing framework docs (all 7 files in `testing-framework/`) against actual implementation:
   - Do the docs describe behavior that is actually implemented?
   - Any contradictions between docs and code?
5. Cross-commit consistency: do the 7 commits (4d16a9f through cd91a8c) reference each other's files/paths correctly?
6. Report: submodules lifecycle results, AGENTS.md accuracy table, doc consistency findings, cross-commit issues.

**Evidence to capture:**
- Submodules setup/update/clean results
- `.gitignore` managed section confirmation
- AGENTS.md skill map accuracy table (skill | exists? | issues)
- Testing framework docs consistency findings
- Cross-commit reference issues

---

## Wave 2: QA Report + Evidence Contract

**Prerequisite:** Wave 1 complete (need build + scripts confirmed working).
**Mode:** Sequential, single agent.
**Time:** ~45 min

### Steps

1. Run `init_report.sh` to generate a report from `.qa-test-report-template.md`.
2. Run `collect_versions.sh` and populate the Run Metadata section.
3. Fill Build & Smoke section using `make build && ./cre version && ./cre --help`.
4. Fill Init section using template compat results from Agent A.
5. Compare report sections against `runbook-phase-map.md` for alignment.
6. Validate evidence contract compliance:
   - PASS/FAIL/SKIP/BLOCKED semantics used correctly
   - Evidence blocks contain: what ran, preconditions, commands, output snippet
   - Summary-first style (summary table before deep logs)
   - Failure taxonomy codes (BLOCKED_ENV, FAIL_COMPAT, etc.) documented in `reporting-rules.md`
   - No huge inline log dumps
   - No raw secrets in output
7. Count PASS/FAIL/SKIP/BLOCKED per section for summary totals.

**Evidence to capture:**
- Generated report file (attach as artifact)
- Evidence contract compliance checklist (per rule: compliant / gap)
- Section alignment with runbook-phase-map

---

## Wave 3: Gap Register + Final Report

**Prerequisite:** All waves complete.
**Mode:** Single operator or agent compiling results.
**Time:** ~1-2 hr

### Inputs

Collect all evidence from:
- Agent A: merge gate results
- Agent B: skills/scripts results
- Agent C: CI/CD + Playwright findings
- Agent D: submodules + docs accuracy
- Wave 2: QA report + evidence contract compliance

### Steps

1. Build the Gap Register table:
   | # | Gap | Severity | Impact | Workaround | Suggested Fix | Owner |

   Seed with known gaps:
   - P0: `playwright-cli` skill referenced in AGENTS.md but doesn't exist
   - P1: Nightly SDK matrix workflow not implemented (design-only)
   - P1: AI validation workflow not implemented (design-only)
   - P1: Go PTY test wrapper not implemented (design-only)
   - P2: macOS not in CI matrix
   - P2: Path filter may miss `internal/` changes
   - P2: Branch protection required-check status unknown

   Add any new gaps discovered during Waves 1-2.

2. Write the final report following the 12-section structure in `validation-and-report-plan.md`.

3. Attach all raw evidence as appendices.

4. Validate the adoption playbook (3-tier plan from the brief) against actual findings -- update time estimates based on validation experience.

---

## Collision Risks & Mitigations

| Risk | Agents | Mitigation |
|------|--------|------------|
| Both run `cre init` creating conflicting directories | A + B | Agent A uses `t.TempDir()` (Go test isolation). Agent B should use a dedicated temp directory for expect scripts. |
| Both modify `cre-templates/` | A + D | Agent A doesn't touch submodules. Agent D owns the submodules lifecycle. No conflict. |
| Script output files collide | B + Wave 2 | Agent B captures output; Wave 2 creates a fresh report. Run Wave 2 after Wave 1. |
| CI/CD inspection triggers workflows | C | Agent C is read-only (local YAML inspection, no pushes). No risk. |

---

## Quick Reference: What Each Wave Produces

| Wave | Output | Used By |
|------|--------|---------|
| 0 | Built `cre` binary, `cre-templates/` dir, tool inventory | Waves 1-2 |
| 1A | Merge gate evidence (per-template, canary, path filter, E2E) | Wave 3 report sections 3 |
| 1B | Skills/scripts evidence table | Wave 3 report sections 5-6 |
| 1C | CI/CD config summary, Playwright status | Wave 3 report sections 4, 8 |
| 1D | Submodules results, AGENTS.md audit, docs consistency | Wave 3 report sections 9 |
| 2 | Sample QA report, evidence contract compliance | Wave 3 report section 7 |
| 3 | Final stakeholder report + gap register | Stakeholder handoff |
