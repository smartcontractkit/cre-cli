---
name: cre-private-registry-testing
description: Tests the CRE CLI private (off-chain) registry feature in staging — workflow deploy, list, simulate, pause, activate, delete, and update against the private registry without a web3 key. Use when asked to test private registry, off-chain workflow deployment, or staging registry lifecycle behavior.
---

# CRE CLI Private Registry Testing

## Context

"Private" workflow management allows users to deploy and manage workflows using only Auth — no web3 key, gas, or RPC needed. The private registry is a Chainlink-hosted off-chain registry. This suite validates CLI commands against it in `staging`.

## Core Workflow

1. Verify prerequisites and env from `references/setup.md`.
2. Execute test cases in order from `references/test-cases.md`.
3. For each case: run the command, validate against the success criteria, record result and evidence.
4. UI-verification steps (workflow tagged `private`, execution logs visible) require a browser session — mark `SKIP_MANUAL` if unavailable.
5. Continue after failures; record expected vs actual and any known issues/tickets.
6. Produce a per-case PASS/FAIL/SKIP/BLOCKED summary with evidence.

## Environment

Prefix every command with `CRE_CLI_ENV=STAGING` or export it:

```bash
export CRE_CLI_ENV=STAGING
cre login   # authenticate against staging before running cases
```

## Rules

- Never print actual secret or credential values — report set/unset only.
- Do NOT set `CRE_ETH_PRIVATE_KEY` for positive-path cases; private registry does not require it.
- Use `--target=staging-settings` for all deploy/pause/activate/delete operations.
- Mark cases blocked by missing staging access or missing org permissions as `BLOCKED_ENV` or `BLOCKED_AUTH`.
- For each `FAIL`, note the known Jira ticket if one exists.

## Failure Taxonomy

Use the codes from the main QA runner (`$cre-qa-runner`) reporting rules:
- `FAIL_COMPAT`, `FAIL_RUNTIME`, `FAIL_ASSERT`, `FAIL_NEGATIVE_PATH`
- `BLOCKED_ENV`, `BLOCKED_AUTH`, `SKIP_MANUAL`, `SKIP_PLATFORM`

## Decision Tree

- General pre-release QA across all CLI areas → use `$cre-qa-runner`.
- Off-chain secrets management with `--secrets-auth=browser` → use `$cre-offchain-secrets-testing`.
- Private registry lifecycle (deploy/list/pause/activate/delete) in staging → use this skill.

## Example

Input request:

```text
Test private registry in staging and produce results.
```

Expected outcome:

```text
Ran 9 test cases against the staging private registry,
produced PASS/FAIL/SKIP/BLOCKED results with evidence per case.
```

## References

- Setup and prerequisites: `references/setup.md`
- Test cases and success criteria: `references/test-cases.md`
