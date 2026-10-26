---
"cre-cli": minor
---

`cre workflow pause` and `cre workflow delete` no longer require the project folder. The positional argument now accepts a workflow folder path (unchanged legacy flow), a workflow name, or an on-chain workflow ID, and can be omitted entirely for an interactive picker of your deployed workflows (resolved from the CRE platform, like `cre workflow list`). Folder-less on-chain workflows require `--rpc-url` and `CRE_ETH_PRIVATE_KEY`; new flags `--owner` and `--registry` scope name resolution. MSIG flows (`--unsigned`/`--changeset`) still require the project folder.
