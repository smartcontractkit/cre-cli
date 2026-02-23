# Validation Commands

Run from repo root unless stated otherwise.

## Minimum Required

```bash
make build
make test
```

## Template-Focused

```bash
go test -v -timeout 20m -run TestTemplateCompatibility ./test/
```

If compatibility test file is not present in the branch yet, run the closest existing init/simulate tests:

```bash
go test -v ./test/... -run 'TestInit|TestSimulate|TestTemplate'
```

## Full Confidence (recommended before merge)

```bash
make test-e2e
```

## Pass Criteria

- Build succeeds.
- Updated template is exercised by at least one automated test.
- No failing checks in `scripts/template_gap_check.sh`.
