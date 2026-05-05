# Test Cases — Off-Chain Secrets (Browser Auth)

All commands assume `CRE_CLI_ENV=STAGING` is set, you are authenticated via interactive `cre login` (not API key), and the org has `claim_vault_secret_management_enabled`. Use `--secrets-auth=browser` on all secrets commands. Use `--target=staging-settings` for workflow operations.

**Important:** The browser auth flow uses PKCE OAuth — the CLI opens a localhost callback server (500s timeout) and waits for a browser redirect. Automated runs use playwright-cli to complete this flow.

---

## BLOCK A: Secret Create

### A-1: Create a single secret (off-chain)

**Setup:**
```bash
export API_KEY_VALUE="test-api-key-value"
```

`secret.yaml`:
```yaml
secretsNames:
  API_KEY:
    - API_KEY_VALUE
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser
```

**Success criteria:**
- Secret created without requiring `CRE_ETH_PRIVATE_KEY`.
- CLI output confirms per-secret result: `secret_id=API_KEY, owner=<orgID>, namespace=main`.

**Known issue:** DEVSVCS-4808 — `ETH_PRIVATE_KEY` still required as of 2026-04-28. Record as partial success with ticket reference.

**Classification:** Script (with browser OAuth redirect handled by playwright)

---

### A-2: Create multiple secrets in one payload (up to 10)

**Setup:**
```bash
export SECRET_1="value1"
export SECRET_2="value2"
# ...up to SECRET_10
```

`multi-secret.yaml`:
```yaml
secretsNames:
  SECRET_1:
    - SECRET_1
  SECRET_2:
    - SECRET_2
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create multi-secret.yaml --secrets-auth=browser
```

**Success criteria:**
- All secrets created; per-secret result shown for each.

**Classification:** Script

---

### A-3: Create with missing environment variable (negative path)

**Setup:** Do NOT export the env var referenced in the YAML.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero before browser opens:
  `environment variable "API_KEY_VALUE" for secret "API_KEY" not found; please export it`

**Classification:** Script

---

### A-4: Create with more than 10 secrets (negative path)

**Setup:** `too-many.yaml` with 11 entries in `secretsNames`.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create too-many.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero: `cannot have more than 10 items in a single payload`

**Classification:** Script

---

### A-5: Create with invalid YAML (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create malformed.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero with YAML parse error.

**Classification:** Script

---

### A-6: Create with empty secretsNames (negative path)

`empty.yaml`:
```yaml
secretsNames: {}
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create empty.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero: `YAML must contain a non-empty 'secretsNames' map`

**Classification:** Script

---

### A-7: Create with `--secrets-auth=browser` using API key credentials (negative path)

**Setup:** Authenticate via API key rather than browser login (`CRE_API_KEY` set).

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero before browser opens:
  `this sign-in flow requires an interactive login; API keys are not supported`

**Classification:** Script

---

### A-8: Create with invalid --timeout (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser --timeout=0
```

**Success criteria:**
- CLI exits non-zero: `invalid --timeout: must be greater than 0 and less than ...`

**Classification:** Script

---

### A-9: Create with `--timeout` within valid range

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser --timeout=1h
```

**Success criteria:**
- Secret created; allowlist expiry set to 1h.

**Classification:** Script

---

### A-10: Create with `--secrets-auth=owner-key-signing` without private key (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=owner-key-signing
```

**Success criteria:**
- CLI exits non-zero due to missing `CRE_ETH_PRIVATE_KEY` or owner not linked.

**Classification:** Script

---

## BLOCK B: Secret List

### B-1: List secrets (default namespace)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets list --target=staging-settings --secrets-auth=browser
```

**Success criteria:**
- Output lists the secret created in A-1 by name (not value).
- Format: `secret_id=API_KEY, owner=<orgID>, namespace=main`.

**Known issue:** DEVSVCS-4808.

**Classification:** Script (with browser OAuth redirect)

---

### B-2: List secrets with explicit namespace

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets list --target=staging-settings --secrets-auth=browser --namespace=main
```

**Success criteria:**
- Same output as B-1; explicit `--namespace=main` matches the default.

**Classification:** Script

---

### B-3: List secrets in empty namespace

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets list --target=staging-settings --secrets-auth=browser --namespace=nonexistent
```

**Success criteria:**
- CLI exits zero; output shows `No secrets found` or empty list for that namespace.

**Classification:** Script

---

## BLOCK C: Secret Update

### C-1: Update a secret

**Setup:** Secret from A-1 must exist. Update the env var value:
```bash
export API_KEY_VALUE="updated-value"
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets update secret.yaml --secrets-auth=browser
```

**Success criteria:**
- Secret updated without requiring a web3 key; CLI exits zero.

**Known issue:** DEVSVCS-4808.

**Reference:** https://docs.chain.link/cre/guides/workflow/secrets/using-secrets-deployed#updating-secrets

**Classification:** Script (with browser OAuth redirect)

---

### C-2: Update a non-existent secret (negative path)

**Setup:** `secret-missing.yaml` referencing a secret ID that does not exist.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets update secret-missing.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero or shows per-item failure in response for that secret ID.

**Classification:** Script

---

## BLOCK D: Secret Delete

**Note:** The delete YAML format differs from create/update — it is a list of secret IDs, not a key-value map.

`secrets-to-delete.yaml`:
```yaml
secretsNames:
  - API_KEY
```

### D-1: Delete a secret

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets delete secrets-to-delete.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits zero.
- Secret no longer appears in `cre secrets list`.

**Reference:** https://docs.chain.link/cre/guides/workflow/secrets/using-secrets-deployed#deleting-secrets

**Classification:** Script (with browser OAuth redirect)

---

### D-2: Delete with wrong YAML format (negative path — passing create-style YAML)

`wrong-format.yaml`:
```yaml
secretsNames:
  API_KEY:
    - API_KEY_VALUE
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets delete wrong-format.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero: `check your YAML format for deletion` (delete expects a list, not a map)

**Classification:** Script

---

### D-3: Delete with empty list (negative path)

`empty-delete.yaml`:
```yaml
secretsNames: []
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets delete empty-delete.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero: `YAML must contain a non-empty 'secretsNames' list`

**Classification:** Script

---

### D-4: Delete with more than 10 secrets (negative path)

**Setup:** `too-many-delete.yaml` with 11 secret IDs in the list.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre secrets delete too-many-delete.yaml --secrets-auth=browser
```

**Success criteria:**
- CLI exits non-zero: `cannot have more than 10 items in a single payload`

**Classification:** Script

---

## BLOCK E: Workflow + Secrets Integration

### E-1: Deploy a workflow that reads a secret from the private registry

**Setup:** Workflow source references the secret from A-1. Set `deployment-registry: "private"` in `workflow.yaml`.

**Commands:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
```

**Success criteria:**
- Workflow deploys without `CRE_ETH_PRIVATE_KEY`.
- Owner ADDRESS in CLI matches staging UI.
- Workflow tagged `private` in UI.
- Workflow logs show the secret is fetched and printed during execution.

**Classification:** Script (deploy) + Manual (log verification → `SKIP_MANUAL` if no browser)

---

### E-2: Workflow lifecycle (list, upsert, activate, delete)

Run in sequence on the workflow from E-1:

```bash
CRE_CLI_ENV=STAGING cre workflow list
# make a change to workflow source
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow activate <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow delete <folder> --target=staging-settings --yes
```

**Success criteria:**
- `list`: deployed workflow visible.
- `deploy` (upsert): CLI exits zero; extra deployment entry in UI.
- `activate`: CLI exits zero; workflow active in UI.
- `delete`: CLI exits zero; workflow absent from `cre workflow list`.

**Known failure:** UI may show both deployments as active after upsert (DEVSVCS-4861).

**Classification:** Script + Manual (UI → `SKIP_MANUAL` if no browser)

---

## Summary table template

| Case | Description | Status | Code | Notes |
|------|-------------|--------|------|-------|
| A-1 | Create single secret (browser) | | | DEVSVCS-4808 |
| A-2 | Create multiple secrets (≤10) | | | |
| A-3 | Create — missing env var (negative) | | | |
| A-4 | Create — >10 secrets (negative) | | | |
| A-5 | Create — invalid YAML (negative) | | | |
| A-6 | Create — empty secretsNames (negative) | | | |
| A-7 | Create — API key with browser flow (negative) | | | |
| A-8 | Create — invalid --timeout (negative) | | | |
| A-9 | Create — valid --timeout | | | |
| A-10 | Create — owner-key without private key (negative) | | | |
| B-1 | List secrets (default namespace) | | | DEVSVCS-4808 |
| B-2 | List secrets --namespace=main | | | |
| B-3 | List secrets -- nonexistent namespace | | | |
| C-1 | Update secret | | | DEVSVCS-4808 |
| C-2 | Update non-existent secret (negative) | | | |
| D-1 | Delete secret | | | |
| D-2 | Delete wrong YAML format (negative) | | | |
| D-3 | Delete empty list (negative) | | | |
| D-4 | Delete >10 items (negative) | | | |
| E-1 | Deploy workflow using secret | | | |
| E-2 | Workflow list/upsert/activate/delete | | | DEVSVCS-4861 |
