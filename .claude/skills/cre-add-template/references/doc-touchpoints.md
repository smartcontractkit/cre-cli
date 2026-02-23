# Documentation Touchpoints

Update docs relevant to template creation and usage in the same PR.

## Always Review

- `docs/cre_init.md`
- `docs/cre.md` (if command summary/behavior changed)
- `.qa-developer-runbook.md` (if validation steps changed)
- `.qa-test-report-template.md` (if report structure needs new checks)

## Conditional

- `docs/cre_workflow_simulate.md` if trigger/simulate expectations change.
- `docs/cre_workflow_deploy.md` if deploy behavior differs for the new template.
- Any template README under `cmd/creinit/template/workflow/*/README.md` if present.

## Consistency Checks

- Template IDs and names match code.
- Flag requirements in docs match implemented behavior.
- Example commands are executable and current.
