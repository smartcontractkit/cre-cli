# Template Addition Checklist

## 1) Add Template Artifacts

Required:
- Add files under `cmd/creinit/template/workflow/<folder>/`.
- Ensure template has expected entry files (`main.go`/`main.ts`, workflow config, language-specific support files).

## 2) Register Template

Required file:
- `cmd/creinit/creinit.go`

Checks:
- Unique template ID.
- Correct language bucket.
- Prompt labels and defaults are accurate.

Dynamic mode (branch-gated):
- If the template source is external, record repository/ref/commit and link the companion template repo change.
- Verify any CLI-side registry/selector wiring still maps correctly to template IDs.

## 3) Dependency Policy

Go templates:
- Use exact version pins in Go template initialization paths.

TypeScript templates:
- Use approved package version strategy and avoid uncontrolled drift.

## 4) Test Coverage

Required file:
- `test/template_compatibility_test.go`

Checks:
- Add new template entry in table.
- Update canary expected count if count changed.
- Ensure expected file list and simulate check string are accurate.

## 5) Documentation

Required touchpoints:
- `docs/cre_init.md`
- Template-specific docs if present.
- Runbook and report guidance when behavior expectations changed.

## 6) Verification

- Execute validation commands from `references/validation-commands.md`.
- Run `scripts/template_gap_check.sh` and resolve all failures.
- For dynamic mode, include an explicit compatibility run that captures source mode and fetched ref in evidence.
