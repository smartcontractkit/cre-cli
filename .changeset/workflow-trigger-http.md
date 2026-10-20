---
"cre-cli": minor
---

Add a hidden `cre workflow trigger http <workflow-folder-path>` command that fires a deployed workflow's HTTP trigger through the CRE Gateway. It signs a `workflows.execute` JSON-RPC request with the `CRE_ETH_PRIVATE_KEY` wallet (which must be one of the workflow's authorized keys), resolves the workflow ID from the selected `--target` unless `--workflow-id` is given, accepts `--input` as inline JSON or `@file`, and prints a machine-readable result with `--json`.
