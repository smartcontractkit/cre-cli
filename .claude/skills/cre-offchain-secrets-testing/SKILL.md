---
name: cre-offchain-secrets-testing
description: Tests CRE CLI off-chain (browser-auth) secrets management in staging — create, list, update, and delete secrets without a web3 key using --secrets-auth=browser. Use when asked to test off-chain secrets, browser-auth secrets, or vault secret management without a private key.
---

# CRE CLI Off-Chain Secrets Testing

## Context

By default, CRE secrets are managed via web3 keys. "Private" (off-chain) secret management lets users create, list, update, and delete secrets using only browser auth (`--secrets-auth=browser`) — no `CRE_ETH_PRIVATE_KEY` required. This requires the `claim_vault_secret_management_enabled` feature flag on the org. This suite validates these flows in `staging`.

## Core Workflow

1. Verify prerequisites and org permissions from `references/setup.md`.
2. Execute test cases in order from `references/test-cases.md`.
3. For each case: run the command, validate against success criteria, record result and evidence.
4. UI-verification steps (secret visible in UI, workflow logs show secret fetched) require a browser session — mark `SKIP_MANUAL` if unavailable.
5. Continue after failures; record expected vs actual and any known issues/tickets.
6. Produce a per-case PASS/FAIL/SKIP/BLOCKED summary with evidence.

## Environment

Prefix every command with `CRE_CLI_ENV=STAGING` or export it:

```bash
export CRE_CLI_ENV=STAGING
cre login   # authenticate against staging before running cases
```

The `--secrets-auth=browser` flag is required on all secrets commands to opt into off-chain management.

## Rules

- Never print actual secret or credential values — use placeholder names only.
- Do NOT set `CRE_ETH_PRIVATE_KEY` for positive-path secrets cases; off-chain secrets do not require it.
- Verify the org has `claim_vault_secret_management_enabled` before starting — if missing, mark secrets cases `BLOCKED_AUTH`.
- Use `--target=staging-settings` for workflow-related operations within this suite.
- For each `FAIL`, note any known Jira ticket (e.g., DEVSVCS-4808).

## Known Issues (as of 2026-04-28)

- DEVSVCS-4808: `ETH_PRIVATE_KEY` still required even when `--secrets-auth=browser` is used. Mark affected cases as partial success with this ticket reference.
- DEVSVCS-4861: After workflow update, UI incorrectly shows both deployments as active.

## Failure Taxonomy

Use the codes from the main QA runner (`$cre-qa-runner`) reporting rules:
- `FAIL_AUTH`, `FAIL_ASSERT`, `FAIL_NEGATIVE_PATH`, `FAIL_RUNTIME`
- `BLOCKED_ENV`, `BLOCKED_AUTH`, `SKIP_MANUAL`, `SKIP_PLATFORM`

## Decision Tree

- General pre-release QA across all CLI areas → use `$cre-qa-runner`.
- Private registry lifecycle without secrets → use `$cre-private-registry-testing`.
- Off-chain secrets (browser-auth) operations in staging → use this skill.

## Example

Input request:

```text
Test off-chain secrets management in staging.
```

Expected outcome:

```text
Ran 11 test cases covering secret create/list/update/delete with --secrets-auth=browser,
produced PASS/FAIL/SKIP/BLOCKED results with evidence per case, noted DEVSVCS-4808 where applicable.
```

## References

- Setup and prerequisites: `references/setup.md`
- Test cases and success criteria: `references/test-cases.md`
