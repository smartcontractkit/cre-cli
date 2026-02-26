# Reporting Rules

Use these rules for `.qa-test-report-YYYY-MM-DD.md`.

## Status Values

Use only:
- `PASS`
- `FAIL`
- `SKIP`
- `BLOCKED`

## Failure Taxonomy Codes

Append a taxonomy code to every `FAIL` and `BLOCKED` status to enable filtering, trending, and root-cause analysis.

| Code | Meaning | When to use |
|------|---------|-------------|
| `FAIL_COMPAT` | Template compatibility failure | Template init, build, or simulate produces an unexpected error |
| `FAIL_BUILD` | Build or compilation failure | `make build`, `go build`, `bun install`, or WASM compilation fails |
| `FAIL_RUNTIME` | Runtime or simulation failure | `cre workflow simulate` fails unexpectedly (not compile-only) |
| `FAIL_ASSERT` | Assertion mismatch | Expected output/file missing or content does not match |
| `FAIL_AUTH` | Authentication failure | `cre login`, `cre whoami`, or credential loading fails |
| `FAIL_NETWORK` | Network or API failure | GraphQL, RPC, or external service unreachable |
| `FAIL_SCRIPT` | Script execution failure | Shell/expect script exits non-zero unexpectedly |
| `BLOCKED_ENV` | Environment not available | Required tool, credential, or service missing |
| `BLOCKED_INFRA` | Infrastructure not available | CI runner, VPN, or staging environment unavailable |
| `BLOCKED_DEP` | Upstream dependency blocked | Blocked by another failing test or unmerged PR |
| `SKIP_MANUAL` | Requires manual verification | Cannot be automated; documented for manual tester |
| `SKIP_PLATFORM` | Platform not applicable | Test only applies to a different OS or environment |

**Usage example:**

```markdown
| Test | Status | Code | Notes |
|------|--------|------|-------|
| Template 1 build | FAIL | FAIL_BUILD | go build exits 1: missing module |
| Staging deploy | BLOCKED | BLOCKED_ENV | CRE_API_KEY not set |
| macOS wizard | SKIP | SKIP_PLATFORM | Linux-only CI runner |
```

## Evidence Policy

- Include command output snippets for each executed test group.
- Keep long output concise by including first/last relevant lines.
- For `FAIL`, write expected behavior and actual behavior.
- For `SKIP` and `BLOCKED`, include a concrete reason.
- Use summary-first style: place a summary table before detailed evidence blocks.

## Evidence Block Format

Wrap per-test evidence in a collapsible `<details>` block with a structured header:

```markdown
<details>
<summary>Evidence: [Test Name] — [STATUS]</summary>

**Command:**
\`\`\`bash
[exact command run]
\`\`\`

**Preconditions:**
- [relevant env vars, tool versions, auth state]

**Output (truncated):**
\`\`\`
[first/last relevant lines of output]
\`\`\`

**Expected:** [what should have happened]
**Actual:** [what did happen — only for FAIL]

</details>
```

Rules:
- Every executed test group must have an evidence block.
- Truncate output to the first and last relevant lines; do not inline full logs.
- For `PASS`, the `Expected` and `Actual` fields can be omitted.
- Attach full logs as downloadable artifacts, not inline.

## Metadata Requirements

Fill these fields before testing:
- Date, Tester, Branch, Commit
- OS and Terminal
- Go/Node/Bun/Anvil versions
- CRE environment
- Template source mode; for dynamic mode also include template repo/ref/commit.

## Safety Policy

- Never include raw token or secret values in evidence.
- Redact sensitive values if they appear in logs.
- If a command would expose secrets, record sanitized output only.

## End-of-Run Quality Gates

- Every runbook section executed or explicitly marked `SKIP`/`BLOCKED`.
- Summary table counts match section outcomes.
- Every `FAIL` and `BLOCKED` has a taxonomy code.
- Final verdict set and justified in notes.
