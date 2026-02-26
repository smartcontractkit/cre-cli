# Validation & Stakeholder Report Plan

*Synthesized from four LLM-generated plans, grounded against actual codebase state on `experimental/agent-skills`.*

---

## Ground Truth: Implemented vs. Design-Only

Before validation, be explicit about what exists as running code vs. documentation-only.

| Component | Status | Evidence |
|-----------|--------|----------|
| Template compatibility gate (5/5 + drift canary) | **Implemented** | `test/template_compatibility_test.go` |
| CI path-filtered template-compat job (Linux + Windows) | **Implemented** | `.github/workflows/pull-request-main.yml` lines 16-86 |
| Skills bundle (6 skills, 7 scripts, 2 expect scripts) | **Implemented** | `.claude/skills/` |
| Skill auditor + audit report | **Implemented** | `.claude/skills/skill-audit-report.md` |
| QA report template | **Implemented** | `.qa-test-report-template.md` |
| Submodules workspace lifecycle | **Implemented** | `submodules.yaml` + `scripts/setup-submodules.sh` |
| AGENTS.md with skill map + component map | **Implemented** | `AGENTS.md` |
| Testing framework docs (7 documents) | **Implemented** | `testing-framework/` |
| SDK version matrix nightly workflow | **Design-only** | Described in `04-ci-cd-integration-design.md`; no `sdk-version-matrix.yml` |
| AI validation workflow | **Design-only** | Described in docs; no `ai-validation.yml` |
| Playwright credential bootstrap | **Design-only** | Referenced in brief + setup.md; no skill, no scripts |
| Go PTY test wrapper | **Design-only** | Described in `implementation-plan.md`; no `test/pty_helper_test.go` |
| macOS CI runner | **Design-only** | Mentioned in docs; not in workflow matrix |

---

## What to Validate (7 Streams)

### Stream 1: Merge Gates (Highest Priority)

**Objective:** Prove the deterministic-first contract actually blocks bad code.

| Check | How | Evidence to Capture |
|-------|-----|---------------------|
| All 5 templates pass | `go test -v -timeout 20m -run TestTemplateCompatibility ./test/` | Per-template PASS/FAIL, runtime |
| Drift canary catches mismatch | Temporarily add a fake template 6 to the registry only (no test table entry) and run the test -- expect failure | Canary failure message |
| Drift canary catches removal | Temporarily remove a template entry from the test table -- expect failure | Canary failure message |
| Path filter triggers correctly | PR touching `cmd/creinit/` -> job runs | CI log showing `run_template_compat=true` |
| Path filter skips correctly | PR touching only `docs/` -> job skipped | CI log showing `run_template_compat=false` |
| Merge group always runs | `merge_group` event -> always `true` | Workflow YAML inspection |
| Template 5 compile-only | TS ConfHTTP uses `simulateMode: "compile-only"` -- verify it compiles but runtime-fails as designed | Test output snippet |
| Existing E2E unbroken | `make test-e2e` passes | Test output |

**Gaps to look for:**
- Does path filter miss files that should trigger the compat job? (e.g., `internal/` changes that affect scaffolding)
- Is `ci-test-template-compat` set as a required check in branch protection settings?
- Are mock GraphQL handlers sufficient for all 5 templates?

### Stream 2: CI/CD & Workflow Configuration

**Objective:** Validate what's running, document what's designed but missing.

| Check | How | Evidence |
|-------|-----|----------|
| PR workflow triggers | Open or inspect existing PR | Job list + trigger conditions |
| Linux + Windows matrix | Inspect `ci-test-template-compat` matrix config | YAML snippet |
| Artifact retention | Check if failed runs preserve usable artifacts | Artifact list from a CI run |
| Nightly SDK matrix | Check for `sdk-version-matrix.yml` | **Does not exist** -- document as design-only |
| AI validation workflow | Check for `ai-validation.yml` | **Does not exist** -- document as design-only |
| Required checks config | Verify branch protection settings | Screenshot or settings export |

**Key gap to report:** The nightly and AI workflows are documented designs, not running code. The design in `04-ci-cd-integration-design.md` should be evaluated for implementation readiness.

### Stream 3: Skills & Scripts

**Objective:** Confirm every skill and script is operational and produces expected output.

| Skill | Script / Action | Validation Command | Expected |
|-------|-----------------|--------------------|----------|
| `cre-qa-runner` | `env_status.sh` | `.claude/skills/cre-qa-runner/scripts/env_status.sh` | Reports set/unset for env vars (no raw secrets) |
| `cre-qa-runner` | `collect_versions.sh` | `.claude/skills/cre-qa-runner/scripts/collect_versions.sh` | Go, Node, Bun, Anvil, CRE versions |
| `cre-qa-runner` | `init_report.sh` | `.claude/skills/cre-qa-runner/scripts/init_report.sh` | Creates dated `.qa-test-report-YYYY-MM-DD.md` from template |
| `cre-add-template` | `template_gap_check.sh` | `.claude/skills/cre-add-template/scripts/template_gap_check.sh` | Exits cleanly with no pending template changes |
| `cre-add-template` | `print_next_steps.sh` | `.claude/skills/cre-add-template/scripts/print_next_steps.sh` | Prints accurate checklist |
| `cre-cli-tui-testing` | `pty-smoke.expect` | Authenticate via Playwright first, then: `expect .claude/skills/cre-cli-tui-testing/tui_test/pty-smoke.expect` | Exit 0, "Project created successfully" |
| `cre-cli-tui-testing` | `pty-overwrite.expect` | Authenticate via Playwright first, then: `expect .claude/skills/cre-cli-tui-testing/tui_test/pty-overwrite.expect` | Exit 0, correct No/Yes behavior |
| `using-cre-cli` | Symlink resolution | `ls -la .claude/skills/using-cre-cli/references/` | `@docs` symlink resolves to `docs/` |
| `skill-auditor` | Audit report | Read `.claude/skills/skill-audit-report.md` | Current date, valid findings |

**Prerequisites:**
- Expect scripts require valid credentials. Use Playwright browser auth with `CRE_USER_NAME`/`CRE_PASSWORD` from `.env` before running.
- See `setup.md` "Authentication for TUI tests" section.

**Gaps to look for:**
- Missing tools (`yq`, `expect`) on operator's machine
- Platform differences (Windows path handling, expect availability)
- Timing sensitivity in expect scripts

### Stream 4: Playwright / Browser Automation

**Objective:** Precisely classify the current state -- this is preparation, not shipped automation.

| Check | Finding |
|-------|---------|
| Skill file at `.claude/skills/playwright-cli/` | **Does not exist** |
| Playwright scripts or test files | **None in repo** |
| Referenced in AGENTS.md? | Yes (lines 107, 122) |
| Referenced in setup.md? | Yes (as a required tool) |
| Referenced in the brief? | Yes (Section "Playwright Primitive") |

**Report framing:** Playwright is a documented design direction with structural hooks (AGENTS.md references, setup.md prerequisite, brief section) but no executable automation was delivered. This aligns with the "Advanced (later)" adoption tier. The AGENTS.md reference to a non-existent skill should be flagged.

### Stream 5: Output & Evidence Contract

**Objective:** Verify outputs follow the evidence contract from the brief.

| Contract | How to Validate | Evidence |
|----------|-----------------|----------|
| PASS/FAIL/SKIP/BLOCKED semantics | Check `reporting-rules.md` + generated report | Terms used correctly |
| Evidence block format | Run `init_report.sh`, fill a section | Contains: what ran, preconditions, commands, output snippet |
| Summary-first style | Check report template structure | Summary table before deep logs |
| Failure taxonomy codes | Check `reporting-rules.md` for BLOCKED_ENV, FAIL_COMPAT etc. | Codes documented and usable |
| Raw logs in artifacts, not inline | Check report template guidance | No huge inline dumps |
| Secret hygiene | Run `env_status.sh` | No raw tokens/keys printed |

### Stream 6: QA Report Pipeline

**Objective:** Produce an actual report and validate its quality.

| Step | Command | Validation |
|------|---------|------------|
| Generate report from template | `init_report.sh` | File created with correct headers |
| Fill Run Metadata | `collect_versions.sh` output -> report | Date, OS, versions, branch, commit present |
| Fill Build & Smoke section | `make build && ./cre version && ./cre --help` | Evidence block populated |
| Fill Init section | Run template compat or manual init | Per-template evidence |
| Check section alignment | Compare report sections to `runbook-phase-map.md` | Sections map correctly |
| Summary totals | Count PASS/FAIL/SKIP/BLOCKED per section | Totals consistent |

### Stream 7: Submodules & Documentation Accuracy

**Objective:** Validate workspace lifecycle and documentation correctness.

| Check | How | Evidence |
|-------|-----|---------|
| Submodules setup | `make setup-submodules` | `cre-templates/` directory created |
| Submodules update | `make update-submodules` | Directory updated, no errors |
| Submodules clean | `make clean-submodules` | Directory removed |
| `.gitignore` managed section | Check `.gitignore` after setup | Managed section present |
| `yq` dependency | Run without `yq` installed | Clear error message |
| AGENTS.md skill map accuracy | Compare listed skills to `.claude/skills/` | All listed skills exist (except `playwright-cli`) |
| AGENTS.md component map | Verify paths and relationships | Paths resolve correctly |
| Testing framework docs consistency | Cross-check 7 docs against implementation | Docs describe actual behavior |
| Cross-commit consistency | Verify 7 commits reference each other correctly (paths, file names) | No broken cross-references |

---

## Execution Order

Optimized for fast failure detection and dependency flow:

| Phase | Stream | Time Est. | Depends On |
|-------|--------|-----------|------------|
| 1 | Environment + Build (`make build`, tool checks, `make setup-submodules`) | 30 min | Nothing |
| 2 | Stream 1: Merge gates (template compat + drift canary + E2E) | 1 hr | Phase 1 |
| 3 | Stream 3: Skills & scripts (all 7 scripts + 2 expect scripts) | 1 hr | Phase 1 |
| 4 | Stream 6: QA report pipeline (generate + fill subset) | 1 hr | Phase 1 |
| 5 | Stream 5: Output contract validation (check report against evidence rules) | 30 min | Phase 4 |
| 6 | Stream 2: CI/CD configuration audit | 30 min | Nothing |
| 7 | Stream 4: Playwright status classification | 15 min | Nothing |
| 8 | Stream 7: Submodules + documentation accuracy + cross-commit check | 1 hr | Phase 1 |
| 9 | Gap register + report writing | 1-2 hr | All above |

**Total estimated: 6-8 hours** (validates/revises the previous plan's 5-7 hour estimate)

---

## Stakeholder Report Structure

```
# CRE CLI Testing Framework — Validation Report & Stakeholder Handoff

## 1. Executive Summary (1 page)
   - What was delivered vs. what remains design-only (table)
   - Validation outcome: PASS / PASS_WITH_GAPS / FAIL
   - Top 3 risks and recommended immediate actions
   - Coverage: 3/5 → 5/5 templates now deterministically validated

## 2. Implemented vs. Design-Only Deliverables
   Table: component | status | commit | validation result
   (Reuse the "Ground Truth" table above, enriched with results)

## 3. Merge Gate Validation
   - Template compatibility: all 5 results
   - Drift canary: positive + negative control evidence
   - Path filter: trigger/skip evidence
   - Branch protection status
   - Gaps found

## 4. CI/CD Validation
   - PR workflow: confirmed working
   - Matrix coverage: Linux + Windows
   - Nightly SDK matrix: designed, not implemented (cite design doc)
   - AI validation workflow: designed, not implemented
   - Artifact retention status

## 5. Skills & Scripts Validation
   Per-skill/script table:
   | Skill | Script | Result | Platform Notes | Gaps |

## 6. TUI / Expect Scripts
   - pty-smoke.expect: result + timing notes
   - pty-overwrite.expect: result + timing notes
   - Go PTY wrapper: designed, not implemented
   - Cross-platform notes

## 7. QA Report Pipeline
   - Report generation: init_report.sh result
   - Metadata capture: collect_versions.sh result
   - Environment status: env_status.sh result
   - Sample report attached/referenced
   - Evidence contract compliance check

## 8. Playwright Status
   - Current: preparation-only (no skill, no scripts)
   - Structural hooks in place (AGENTS.md, setup.md, brief)
   - Recommendation for next steps

## 9. Submodules & Documentation
   - Workspace lifecycle: setup/update/clean results
   - AGENTS.md accuracy audit results
   - Cross-commit consistency results
   - Testing framework docs accuracy

## 10. Gap Register
   Prioritized table:
   | # | Gap | Severity | Impact | Workaround | Suggested Fix | Owner |

   Known gaps to seed:
   - P0: playwright-cli skill referenced but doesn't exist
   - P1: Nightly SDK matrix workflow not implemented
   - P1: AI validation workflow not implemented
   - P1: Go PTY test wrapper not implemented
   - P2: macOS not in CI matrix
   - P2: Path filter may miss internal/ changes
   - P2: Branch protection required-check status unknown

## 11. Adoption Playbook (Validated)
   Restate the 3-tier plan from the brief with validation notes:
   - Minimum (1-2 days): what's ready now
   - Recommended (1-2 weeks): what needs setup
   - Advanced (later): what remains design-only
   Include updated time estimates based on validation experience.

## 12. Takeover Checklist
   - Repo state (branch, PR link)
   - Required tools and dependencies
   - Commands to run on day 1
   - Monthly maintenance tasks
   - "When adding template N+1" checklist (reference cre-add-template skill)
   - Ownership boundaries

## Appendix
   A. Raw test output logs
   B. Sample QA report
   C. Environment details (OS, tool versions)
   D. Time spent per validation phase
   E. CI run links (if applicable)
```

---

## Quick Reference: Validation Commands

```bash
# Phase 1: Environment + Build
make build && ./cre version && ./cre --help
make setup-submodules
command -v go expect bun node forge anvil
go version && bun --version && node -v && forge --version && anvil --version

# Phase 2: Merge Gates
go test -v -timeout 20m -run TestTemplateCompatibility ./test/
make test-e2e

# Phase 3: Skills & Scripts
.claude/skills/cre-qa-runner/scripts/env_status.sh
.claude/skills/cre-qa-runner/scripts/collect_versions.sh
.claude/skills/cre-qa-runner/scripts/init_report.sh
.claude/skills/cre-add-template/scripts/template_gap_check.sh
.claude/skills/cre-add-template/scripts/print_next_steps.sh
expect .claude/skills/cre-cli-tui-testing/tui_test/pty-smoke.expect
expect .claude/skills/cre-cli-tui-testing/tui_test/pty-overwrite.expect

# Phase 8: Submodules lifecycle
make setup-submodules
make update-submodules
make clean-submodules
```
