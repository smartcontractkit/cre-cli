---
"cre-cli": major
---

Breaking change: generated TypeScript EVM `writeReportFrom*` helpers now sign only ABI-encoded arguments by default, excluding the 4-byte function selector. If your receiver expects the legacy full-calldata payload, regenerate your bindings with `cre generate-bindings evm --include-function-selector`. Go bindings are unaffected.
