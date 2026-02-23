---
name: cre-qa-runner
description: Runs the CRE CLI pre-release QA runbook end-to-end and produces a structured report from the local template, including branch-gated dynamic template pull validation when available. Use when the user asks to run QA, perform pre-release validation, test the CLI end-to-end, or generate a QA report.
---

# CRE CLI QA Runner

## Core Workflow

1. Verify prerequisites first: run `scripts/env_status.sh` and `scripts/collect_versions.sh`, and report only env var set/unset status.
2. Initialize a dated report file with `scripts/init_report.sh` before executing any runbook step.
3. Execute phases from `references/runbook-phase-map.md` in order, mapping each action to the matching section in the report.
4. Use command guidance from `$using-cre-cli` and PTY traversal guidance from `$cre-cli-tui-testing` when a phase requires them.
5. Capture template source mode in evidence (embedded baseline or dynamic pull branch mode) and include provenance for dynamic mode.
6. Classify each case as Script, AI-interpreted, or Manual-only using `references/manual-only-cases.md`.
7. Continue after failures, record evidence, and produce final PASS/FAIL/SKIP/BLOCKED totals.

## Rules

- Never print secret values; report only set/unset status for sensitive env vars.
- Do not edit `.qa-test-report-template.md`; always copy it to a dated report file.
- For each failure, record expected vs actual behavior and continue to remaining phases unless blocked by a hard dependency.
- Mark truly unexecutable cases as `BLOCKED` with a concrete reason.

## Failure Handling

- If prerequisite tooling is missing, mark affected phases `BLOCKED` and record the missing tool/version.
- If auth is unavailable for deploy/secrets flows, mark dependent cases `BLOCKED` and continue with non-auth phases.
- If a command fails unexpectedly, capture output evidence and continue to the next runnable case.

## Output Contract

- Report path: `.qa-test-report-YYYY-MM-DD.md` at repo root.
- Report content rules: follow `references/reporting-rules.md` exactly.
- Include run metadata, per-section status, evidence blocks, failures, and a final summary verdict.
- When dynamic template mode is used, include template repo/ref/commit metadata in the run report.

## Decision Tree

- If the request is command syntax, flags, or a single command behavior question, use `$using-cre-cli` instead.
- If the request is specifically interactive wizard traversal or auth-gated TUI prompt testing, use `$cre-cli-tui-testing` instead.
- If the request is release or pre-release QA evidence generation across multiple CLI areas, use this skill.

## Example

Input request:

```text
Run pre-release QA for this branch and produce the QA report.
```

Expected outcome:

```text
Created .qa-test-report-2026-02-20.md, executed runbook phases in order,
filled section statuses with evidence, and produced final verdict summary.
```

## References

- Runbook phase mapping and evidence policy: `references/runbook-phase-map.md`
- Report field and status rules: `references/reporting-rules.md`
- Manual-only and conditional skip guidance: `references/manual-only-cases.md`
