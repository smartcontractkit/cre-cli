# Reporting Rules

Use these rules for `.qa-test-report-YYYY-MM-DD.md`.

## Status Values

Use only:
- `PASS`
- `FAIL`
- `SKIP`
- `BLOCKED`

## Evidence Policy

- Include command output snippets for each executed test group.
- Keep long output concise by including first/last relevant lines.
- For `FAIL`, write expected behavior and actual behavior.
- For `SKIP` and `BLOCKED`, include a concrete reason.

## Metadata Requirements

Fill these fields before testing:
- Date, Tester, Branch, Commit
- OS and Terminal
- Go/Node/Bun/Anvil versions
- CRE environment
- Template source mode; for dynamic mode also include template repo/ref/commit.

## Safety Policy

- Never include raw token or secret values in evidence.
- Redact sensitive values if they appear in logs.
- If a command would expose secrets, record sanitized output only.

## End-of-Run Quality Gates

- Every runbook section executed or explicitly marked `SKIP`/`BLOCKED`.
- Summary table counts match section outcomes.
- Final verdict set and justified in notes.
