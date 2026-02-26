# Skill Audit Report

**Date:** 2026-02-25  
**Scope:** All 6 skills in `.claude/skills/`  
**Method:** Lightweight batch audit across 7 dimensions

---

## Triage Table

| Skill | CRIT | WARN | INFO | Top Issue |
|-------|------|------|------|-----------|
| skill-auditor | 0 | 2 | 1 | Uses SKILLS.md not SKILL.md; 419 lines with embedded checklist |
| playwright-cli | 0 | 1 | 1 | 279 lines, heavy inline command reference |
| using-cre-cli | 0 | 0 | 1 | Command map could move to references/ |
| cre-cli-tui-testing | 0 | 0 | 0 | — |
| cre-qa-runner | 0 | 0 | 0 | — |
| cre-add-template | 0 | 0 | 0 | — |

---

## Directory Contents (per skill)

| Skill | Contents |
|-------|----------|
| **using-cre-cli** | `SKILL.md` only (references `references/@docs` symlink → `../../../../docs`) |
| **cre-cli-tui-testing** | `SKILL.md`, `references/setup.md`, `references/test-flow.md`, `tui_test/pty-smoke.expect`, `tui_test/pty-overwrite.expect` |
| **cre-qa-runner** | `SKILL.md`, `scripts/env_status.sh`, `scripts/collect_versions.sh`, `scripts/init_report.sh`, `references/runbook-phase-map.md`, `references/reporting-rules.md`, `references/manual-only-cases.md` |
| **cre-add-template** | `SKILL.md`, `scripts/template_gap_check.sh`, `scripts/print_next_steps.sh`, `references/validation-commands.md`, `references/template-checklist.md`, `references/doc-touchpoints.md` |
| **playwright-cli** | `SKILL.md`, `references/setup.md`, `references/request-mocking.md`, `references/running-code.md`, `references/session-management.md`, `references/storage-state.md`, `references/test-generation.md`, `references/tracing.md`, `references/video-recording.md` |
| **skill-auditor** | `SKILLS.md` only (no `references/`, no `scripts/`) |

---

## Detailed Findings

### 1. skill-auditor

| Dimension | Severity | What was found | Fix suggestion |
|-----------|----------|----------------|----------------|
| Structural Hygiene | WARNING | Uses `SKILLS.md` instead of `SKILL.md`; breaks convention used by all other skills; may affect discovery or tooling expectations | Rename `SKILLS.md` → `SKILL.md` for consistency. If intentional, document why in AGENTS.md. |
| Token Efficiency | WARNING | 419 lines; ~200 lines of embedded "Audit Checklist — Detailed Reference" (lines 115–331) inlined in main file | Move checklist to `references/audit-checklist.md` and keep a short summary + link in SKILL.md. |
| Structural Hygiene | INFO | Line count 419 (approaching 500-line target) | After moving checklist to references/, SKILL.md should drop to ~200 lines. |

**Pattern:** Iterative Refinement + Context-Aware Tool Selection  
**Differentiation:** Meta-skill for auditing other skills; clearly distinct from CRE CLI domain skills.

---

### 2. playwright-cli

| Dimension | Severity | What was found | Fix suggestion |
|-----------|----------|----------------|----------------|
| Token Efficiency | WARNING | 279 lines; ~200 lines of inline command examples (Core, Navigation, Keyboard, Mouse, Storage, Network, DevTools, etc.) | Move full command reference to `references/commands.md`; keep Quick Start + 1–2 examples in SKILL.md. |
| Structural Hygiene | INFO | Line count 279 (>300 threshold for INFO) | Will resolve after moving command reference to references/. |

**Pattern:** Domain-Specific Intelligence (tool reference)  
**Differentiation:** Generic browser automation; CRE-specific usage (login) is in `references/setup.md`. Clearly distinct from CLI command skills (`using-cre-cli`), PTY testing (`cre-cli-tui-testing`), and QA runbook (`cre-qa-runner`).

---

### 3. using-cre-cli

| Dimension | Severity | What was found | Fix suggestion |
|-----------|----------|----------------|----------------|
| Token Efficiency | INFO | Command map (lines 53–89, ~40 lines) is inline; could be moved for progressive disclosure | Consider `references/command-map.md`; keep a short "Core commands" list in SKILL.md and link to full map. |

**Pattern:** Domain-Specific Intelligence  
**Differentiation:** Explicit "Do not use for PTY-specific interactive wizard traversal testing" keeps boundary with `cre-cli-tui-testing`.

---

### 4. cre-cli-tui-testing

No findings. Lean (32 lines), clear structure, good progressive disclosure via `references/` and `tui_test/`.

**Pattern:** Sequential Workflow Orchestration

---

### 5. cre-qa-runner

No findings. Decision Tree (lines 37–41) explicitly defers to `using-cre-cli` and `cre-cli-tui-testing` for narrower requests.

**Pattern:** Sequential Workflow Orchestration

---

### 6. cre-add-template

No findings. Well-structured with scripts and references.

**Pattern:** Sequential Workflow Orchestration

---

## Invocation Overlap Analysis

### Potential overlap: "test cre" / "test the CLI"

| User phrase | Intended skill | Notes |
|-------------|----------------|-------|
| "test the CLI end-to-end" | cre-qa-runner | Explicit trigger in description |
| "test the wizard" / "test interactive flow" | cre-cli-tui-testing | PTY/TUI traversal |
| "test cre commands" | Ambiguous | Could mean: (a) run commands to verify behavior → `using-cre-cli`, or (b) PTY traversal → `cre-cli-tui-testing` |
| "how do I run cre init?" | using-cre-cli | Command syntax |
| "run QA and produce report" | cre-qa-runner | Explicit |

**Mitigation:** Boundaries are already clear:
- `cre-cli-tui-testing`: "Keep general command syntax questions in $using-cre-cli"
- `cre-qa-runner`: Decision Tree defers single-command questions to `using-cre-cli` and wizard-only testing to `cre-cli-tui-testing`

**Recommendation:** No change required. If "test cre commands" ambiguity causes issues, add to `using-cre-cli` description: "Use when the user asks to run or verify cre commands (syntax, flags, single-command behavior). Do not use for PTY/TUI wizard traversal testing."

---

### playwright-cli vs CRE skills

| User phrase | Intended skill |
|-------------|----------------|
| "navigate to CRE login page" / "fill the OAuth form" | playwright-cli |
| "how do I run cre login?" | using-cre-cli |
| "test the wizard with browser auth" | cre-cli-tui-testing (which delegates browser steps to playwright-cli) |

**Differentiation:** Clear. playwright-cli = browser automation; CRE skills = CLI/terminal/QA.

---

### skill-auditor vs others

| User phrase | Intended skill |
|-------------|----------------|
| "audit my skills" / "check my skills" / "why isn't my skill triggering" | skill-auditor |
| "run cre commands" / "add a template" / "run QA" | CRE skills |

**Differentiation:** Fully distinct; no overlap.

---

## Summary

- **CRITICAL:** 0  
- **WARNING:** 3 (skill-auditor: 2; playwright-cli: 1)  
- **INFO:** 3 (skill-auditor: 1; playwright-cli: 1; using-cre-cli: 1)

**Top actions:**
1. Rename `skill-auditor/SKILLS.md` → `SKILL.md` (or document exception in AGENTS.md).
2. Move skill-auditor checklist to `references/audit-checklist.md`.
3. Move playwright-cli command reference to `references/commands.md`.

**Validation method:** Description scan, structural review, and overlap simulation across all 6 skills.
