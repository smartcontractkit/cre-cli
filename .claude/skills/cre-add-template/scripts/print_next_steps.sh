#!/usr/bin/env bash
set -euo pipefail

cat <<'OUT'
## Template Addition Next Steps

- [ ] Confirm template files are present under `cmd/creinit/template/workflow/<folder>/`
- [ ] Confirm template registration in `cmd/creinit/creinit.go`
- [ ] Confirm `test/template_compatibility_test.go` includes the new template and canary count
- [ ] Confirm docs updates in `docs/` and runbook/report touchpoints as needed
- [ ] Run: `make build`
- [ ] Run: `make test`
- [ ] Run: `go test -v -timeout 20m -run TestTemplateCompatibility ./test/`
- [ ] Run (recommended): `make test-e2e`
- [ ] Run: `.claude/skills/cre-add-template/scripts/template_gap_check.sh`
OUT
