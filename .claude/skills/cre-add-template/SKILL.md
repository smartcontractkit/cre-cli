---
name: cre-add-template
description: Guides the end-to-end CRE CLI template addition workflow and enforces required registry, test, and docs updates across embedded templates and upcoming dynamic template-repo flows. Use when the user asks to add a template, scaffold a new template, register template IDs, or update template tests/docs after template changes.
---

# CRE Add Template

## Core Workflow

1. Decide source mode first: embedded template edits in this repo vs branch-gated dynamic template-repo edits.
2. Create template files under `cmd/creinit/template/workflow/<folder>/` for embedded mode, or apply equivalent edits in the external template repo for dynamic mode.
3. Register the template in `cmd/creinit/creinit.go` with correct language, template ID, and prompt metadata.
4. Apply dependency policy: Go templates use exact pins; TypeScript templates should avoid accidental drift and use approved version strategy.
5. Update template coverage in `test/template_compatibility_test.go` (add table entry and update canary count if needed).
6. Update user docs in `docs/` and runbook touchpoints listed in `references/doc-touchpoints.md`.
7. Run validation commands from `references/validation-commands.md`.
8. Run `scripts/template_gap_check.sh` and include `scripts/print_next_steps.sh` output in the PR summary.

## Rules

- Do not merge template additions without a compatibility test update.
- Keep template ID mapping and test table in sync.
- Update docs in the same change set as code.
- If a new template introduces interactive behavior, ensure PTY/TUI coverage is explicitly assessed.
- For dynamic mode (branch-gated), include CLI-template compatibility evidence and template ref/commit provenance in the change notes.

## Failure Handling

- If registry updates and template files diverge, stop and reconcile IDs before running tests.
- If compatibility tests fail, fix template scaffolding or expected-file assertions before proceeding.
- If docs are missing, do not close the task; run `scripts/template_gap_check.sh` until all required categories pass.

## Required Outputs

- New template files committed.
- Registry update committed.
- Compatibility test update committed.
- Documentation updates committed.
- Validation results captured.

## Example

Input request:

```text
Add a new TypeScript template for webhook ingestion and wire it into cre init.
```

Expected outcome:

```text
Template files added under cmd/creinit/template/workflow/, template registered in
cmd/creinit/creinit.go, compatibility tests updated, docs updated, and validation
commands executed with results recorded.
```

## References

- Canonical checklist: `references/template-checklist.md`
- Validation commands and pass criteria: `references/validation-commands.md`
- Required doc touchpoints: `references/doc-touchpoints.md`
