---
"cre-cli": patch
---

`cre secrets create`, `update`, `delete` and `list` now exit non-zero when the Vault rejects a request (for example "key does not exist" on update). Previously the failure was only printed and the command exited 0, so scripts and CI treated it as success.
