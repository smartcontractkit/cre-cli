---
"cre-cli": minor
---

Add `--output json` / `--json` to `cre whoami`, `cre account list-key` and `cre secrets list` so their results can be consumed by scripts. JSON is the only thing written to stdout (progress and status messages go to stderr), `cre account list-key` returns `[]` when no owners are linked, and `cre secrets list` returns `{"owner", "namespace", "secrets": [...]}` and exits non-zero when the vault reports a failure.
