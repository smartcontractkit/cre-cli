---
name: using-cre-cli
description: Provides guidance for operating the CRE CLI for project setup, authentication, account key management, workflow deployment and lifecycle, secret management, versioning, bindings generation, and template-source troubleshooting from local CRE docs. Use when the user asks to run or troubleshoot cre commands, requests command syntax or flags, or asks command-level behavior questions for workflows, secrets, account operations, or dynamic template pull command paths. Do not use for PTY-specific interactive wizard traversal testing.
---

# Using CRE CLI

## Quick Start

```bash
# show top-level help and global flags
cre --help

# check current auth state
cre whoami

# initialize a project
cre init

# list workflows or run workflow actions
cre workflow --help

# manage secrets
cre secrets --help
```

## Operating Workflow

1. Confirm scope: identify whether the request is about setup, auth, account keys, workflows, secrets, bindings, or versioning.
2. Read the relevant docs in `references/@docs/` before running commands with non-trivial flags.
3. Prefer exact command examples from docs, then adapt only the parts required by user inputs.
4. Verify prerequisites explicitly for mutating operations (`deploy`, `activate`, `pause`, `delete`, `secrets create/update/delete`).
5. After execution, report the command run, key output, and immediate next checks.

## Template Source Mode Handling

- Current behavior: `cre init` scaffolding is driven by embedded templates in this repo.
- Branch-gated upcoming behavior: dynamic template pull flows may add source/ref flags or config.
- For dynamic-mode requests, first confirm whether the branch/flag set exists locally, then provide command guidance for that branch-specific interface.
- If dynamic-template fetch fails, troubleshoot in this order: auth, repo/ref selection, network reachability, then cache/workdir state.

## Documentation Access

- The skill references the repository docs via symlink: `references/@docs -> ../../../../docs`.
- Use `rg` to locate flags/examples quickly:

```bash
rg -n "^## |^### |--|Synopsis|Examples" .claude/skills/using-cre-cli/references/@docs/*.md
```

## Command Map

### Core

- `cre`: [references/@docs/cre.md](references/@docs/cre.md)
- `cre init`: [references/@docs/cre_init.md](references/@docs/cre_init.md)
- `cre version`: [references/@docs/cre_version.md](references/@docs/cre_version.md)
- `cre update`: [references/@docs/cre_update.md](references/@docs/cre_update.md)
- `cre generate-bindings`: [references/@docs/cre_generate-bindings.md](references/@docs/cre_generate-bindings.md)

### Authentication

- `cre login`: [references/@docs/cre_login.md](references/@docs/cre_login.md)
- `cre logout`: [references/@docs/cre_logout.md](references/@docs/cre_logout.md)
- `cre whoami`: [references/@docs/cre_whoami.md](references/@docs/cre_whoami.md)

### Account Key Management

- `cre account`: [references/@docs/cre_account.md](references/@docs/cre_account.md)
- `cre account link-key`: [references/@docs/cre_account_link-key.md](references/@docs/cre_account_link-key.md)
- `cre account list-key`: [references/@docs/cre_account_list-key.md](references/@docs/cre_account_list-key.md)
- `cre account unlink-key`: [references/@docs/cre_account_unlink-key.md](references/@docs/cre_account_unlink-key.md)

### Workflow Lifecycle

- `cre workflow`: [references/@docs/cre_workflow.md](references/@docs/cre_workflow.md)
- `cre workflow deploy`: [references/@docs/cre_workflow_deploy.md](references/@docs/cre_workflow_deploy.md)
- `cre workflow activate`: [references/@docs/cre_workflow_activate.md](references/@docs/cre_workflow_activate.md)
- `cre workflow pause`: [references/@docs/cre_workflow_pause.md](references/@docs/cre_workflow_pause.md)
- `cre workflow delete`: [references/@docs/cre_workflow_delete.md](references/@docs/cre_workflow_delete.md)
- `cre workflow simulate`: [references/@docs/cre_workflow_simulate.md](references/@docs/cre_workflow_simulate.md)

### Secrets Lifecycle

- `cre secrets`: [references/@docs/cre_secrets.md](references/@docs/cre_secrets.md)
- `cre secrets create`: [references/@docs/cre_secrets_create.md](references/@docs/cre_secrets_create.md)
- `cre secrets update`: [references/@docs/cre_secrets_update.md](references/@docs/cre_secrets_update.md)
- `cre secrets delete`: [references/@docs/cre_secrets_delete.md](references/@docs/cre_secrets_delete.md)
- `cre secrets list`: [references/@docs/cre_secrets_list.md](references/@docs/cre_secrets_list.md)
- `cre secrets execute`: [references/@docs/cre_secrets_execute.md](references/@docs/cre_secrets_execute.md)

## Execution Rules

- Use `cre --help` and command-specific `--help` when flags are uncertain.
- Preserve user-provided environment/target options (`-e`, `-R`, `-T`) when present.
- For destructive operations, confirm identifiers and environment before execution.
- When troubleshooting, reproduce with the smallest command first, then add flags incrementally.
