---
"cre-cli": patch
---

Warn about private-key (EOA) ownership on every command that signs with `CRE_ETH_PRIVATE_KEY`, not just `workflow deploy`. `workflow pause`, `workflow activate`, `workflow delete`, `account link-key`, and `secrets create/update/delete/list` now print a warning that suggests multi-sig ownership with `--unsigned` or `--changeset`.
