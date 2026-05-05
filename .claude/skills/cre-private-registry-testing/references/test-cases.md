# Test Cases — Private Registry

All commands assume `CRE_CLI_ENV=STAGING` is set and you are authenticated via `cre login`.
Use `--target=staging-settings` for all workflow operations unless otherwise noted.

---

## BLOCK A: Registry

### A-1: List registries

**Command:**
```bash
CRE_CLI_ENV=STAGING cre registry list
```

**Success criteria:**
- Output lists both registries:
  - `onchain:ethereum-testnet-sepolia` → type `on-chain`, with address
  - `private` → type `off-chain`, no address

**Classification:** Script

---

## BLOCK B: Deploy

### B-1: Deploy a workflow to the private registry

**Setup:**
```bash
CRE_CLI_ENV=STAGING cre init   # choose any CRON-based template
# if TypeScript: cd <folder> && bun install
```

Add to `workflow.yaml`:
```yaml
deployment-registry: "private"
```

**Commands:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
```

**Success criteria:**
- Workflow deploys without requiring `CRE_ETH_PRIVATE_KEY`.
- Owner ADDRESS in CLI output matches the address in the staging UI.
- Workflow visible in staging UI; tagged as `private`; executing and showing logs.

**Classification:** Script (deploy/simulate) + Manual (UI verification → `SKIP_MANUAL` if no browser)

---

### B-2: Deploy without config file (`--no-config`)

**Setup:** Start from the workflow folder from B-1.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --no-config --target=staging-settings
```

**Success criteria:**
- Deploys without reading any config file.
- CLI exits zero; deployment appears in staging UI.

**Note:** `--config`, `--no-config`, and `--default-config` are mutually exclusive — attempting two at once should error.

**Classification:** Script

---

### B-3: Deploy with pre-built WASM (`--wasm`)

**Setup:** Build WASM from the workflow folder first:
```bash
CRE_CLI_ENV=STAGING cre workflow build <folder>
```

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --wasm=./binary.wasm.br.b64 --target=staging-settings
```

**Success criteria:**
- Compilation step is skipped.
- CLI deploys from the provided WASM binary; exits zero.

**Classification:** Script

---

### B-4: Deploy with `CRE_ETH_PRIVATE_KEY` set (negative path)

**Setup:** Set `CRE_ETH_PRIVATE_KEY` to an invalid value (e.g., `y`).

**Command:**
```bash
CRE_ETH_PRIVATE_KEY=y CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits non-zero with error:
  `failed to load settings: failed to parse private key. Please check CRE_ETH_PRIVATE_KEY`

**Classification:** Script

---

### B-5: Deploy with mutually exclusive config flags (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --config=config.yaml --no-config --target=staging-settings
```

**Success criteria:**
- CLI exits non-zero; error indicates `--config`, `--no-config`, and `--default-config` are mutually exclusive.

**Classification:** Script

---

## BLOCK C: Upsert (Re-deploy)

### C-1: Upsert a workflow (deploy a changed version)

**Setup:** Make a small code change to the workflow from B-1. Redeploy.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits zero.
- Staging UI shows an additional deployment entry for this workflow.

**Known failure:** UI may incorrectly show both deployments as active (DEVSVCS-4861) — record as `FAIL_ASSERT` with ticket reference if observed.

**Classification:** Script (deploy) + Manual (UI verification → `SKIP_MANUAL` if no browser)

---

## BLOCK D: List

### D-1: List all workflows

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow list
```

**Success criteria:**
- Output lists the deployed workflow from B-1.
- Private-registry workflow is identifiable in the output.

**Classification:** Script

---

### D-2: List workflows filtered by registry

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow list --registry=private
```

**Success criteria:**
- Output contains only workflows deployed to the private registry.
- On-chain workflows are excluded.

**Negative:** `--registry=invalid-id` must exit non-zero:
```
registry "invalid-id" not found in user context; available: [...]
```

**Classification:** Script

---

### D-3: List workflows including deleted

**Setup:** Delete a workflow (see Block F), then run:

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow list --include-deleted
```

**Success criteria:**
- Deleted workflow appears in output.
- Without `--include-deleted` the same workflow is absent.

**Classification:** Script

---

### D-4: List workflows as JSON

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow list --output=json
```

**Success criteria:**
- Output is a valid JSON array.

**Negative:** `--output=csv` must exit non-zero:
```
--output "csv" is not supported; only "json" is accepted
```

**Classification:** Script

---

## BLOCK E: Pause and Activate

### E-1: Pause a workflow

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow pause <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits zero.
- Staging UI shows workflow status as paused.

**Reference:** https://docs.chain.link/cre/guides/operations/activating-pausing-workflows

**Classification:** Script (command) + Manual (UI → `SKIP_MANUAL` if no browser)

---

### E-2: Pause an already-paused workflow (negative path)

**Setup:** Pause the workflow first (E-1), then run pause again.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow pause <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits non-zero with error:
  `workflow is already paused, cancelling transaction`

**Classification:** Script

---

### E-3: Activate a workflow

**Setup:** Workflow must be paused (run E-1 first).

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow activate <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits zero.
- Staging UI shows workflow as active.

**Known edge case:** If a workflow is paused and you deploy a misconfigured version (e.g., trigger every second), the CLI may report active while the engine/UI does not reflect this. Record as a separate observation if encountered.

**Reference:** https://docs.chain.link/cre/guides/operations/activating-pausing-workflows

**Classification:** Script (command) + Manual (UI → `SKIP_MANUAL` if no browser)

---

### E-4: Activate an already-active workflow (negative path)

**Setup:** Ensure workflow is active, then run activate.

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow activate <folder> --target=staging-settings
```

**Success criteria:**
- CLI exits non-zero with error:
  `workflow is already active, cancelling transaction`

**Classification:** Script

---

## BLOCK F: Delete

### F-1: Delete a workflow (interactive confirmation)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow delete <folder> --target=staging-settings
```

**Interactive confirmation:** CLI prompts:
```
Are you sure you want to delete the workflow '<name>'?
```
Type the workflow name exactly to confirm.

**Success criteria:**
- CLI exits zero after correct name is typed.
- Workflow no longer appears in `cre workflow list`.

**Classification:** Script (with interactive confirmation prompt)

---

### F-2: Delete a workflow — cancel confirmation (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow delete <folder> --target=staging-settings
```

At the confirmation prompt, type anything other than the exact workflow name (or press Ctrl+C).

**Success criteria:**
- CLI exits with message: `Workflow deletion canceled`
- Workflow still appears in `cre workflow list`.

**Classification:** Script (interactive)

---

### F-3: Delete a workflow non-interactively

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow delete <folder> --target=staging-settings --yes
```

**Success criteria:**
- CLI skips confirmation prompt; exits zero.
- Workflow deleted.

**Classification:** Script

---

### F-4: Delete a workflow that does not exist (negative path)

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow delete <folder-with-nonexistent-name> --target=staging-settings --yes
```

**Success criteria:**
- CLI exits zero with a warning (no workflows found); does not error.

**Classification:** Script

---

## BLOCK G: Simulate

### G-1: Simulate a CRON-triggered workflow

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings
```

**Interactive:** Select trigger when prompted; optionally press Enter to skip the wait.

**Success criteria:**
- Simulator runs; prints workflow name, binary hash, and execution result JSON.

**Classification:** Script (interactive trigger selection)

---

### G-2: Simulate non-interactively with `--trigger-index`

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings --trigger-index=0 --yes
```

**Success criteria:**
- CLI uses trigger at index 0 without prompting; exits zero.

**Negative:** `--trigger-index` out of range must exit non-zero:
```
Invalid --trigger-index <n>; available range: 0-<max>
```

**Classification:** Script

---

### G-3: Simulate with limits disabled

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings --trigger-index=0 --yes --limits=none
```

**Success criteria:**
- Simulation runs with no size or execution-time limits enforced.

**Classification:** Script

---

### G-4: Simulate with pre-built WASM

**Command:**
```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --wasm=./binary.wasm.br.b64 --target=staging-settings --trigger-index=0 --yes
```

**Success criteria:**
- Compilation step is skipped; simulation uses the supplied WASM.

**Classification:** Script

---

## Summary table template

| Case | Description | Status | Code | Notes |
|------|-------------|--------|------|-------|
| A-1 | List registries | | | |
| B-1 | Deploy to private registry | | | |
| B-2 | Deploy --no-config | | | |
| B-3 | Deploy --wasm pre-built | | | |
| B-4 | Deploy with ETH_PRIVATE_KEY set (negative) | | | |
| B-5 | Deploy with mutually exclusive config flags (negative) | | | |
| C-1 | Upsert workflow | | | |
| D-1 | List all workflows | | | |
| D-2 | List filtered by --registry | | | |
| D-3 | List --include-deleted | | | |
| D-4 | List --output=json | | | |
| E-1 | Pause workflow | | | |
| E-2 | Pause already-paused (negative) | | | |
| E-3 | Activate workflow | | | |
| E-4 | Activate already-active (negative) | | | |
| F-1 | Delete with interactive confirmation | | | |
| F-2 | Delete cancel confirmation (negative) | | | |
| F-3 | Delete non-interactively (--yes) | | | |
| F-4 | Delete non-existent workflow (negative) | | | |
| G-1 | Simulate CRON trigger (interactive) | | | |
| G-2 | Simulate non-interactive --trigger-index | | | |
| G-3 | Simulate --limits=none | | | |
| G-4 | Simulate --wasm pre-built | | | |
