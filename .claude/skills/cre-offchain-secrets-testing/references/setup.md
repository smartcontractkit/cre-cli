# Setup

## Prerequisites

- CRE CLI v1.12.0 or later: `cre update` or follow https://docs.chain.link/cre/getting-started/cli-installation
- Tailscale VPN active and connected to @smartcontract.com network
- Access to a staging organization at https://staging-cre-ui-cll.vercel.app/
  - All staging orgs have deploy access by default
- **`claim_vault_secret_management_enabled` feature flag on your org** — required for off-chain secrets
  - Request in the #cre-accounts Slack channel (see example: https://chainlink-core.slack.com/archives/C09A2ME0AJ1/p1777416502133429)
  - Without this flag, secrets default to web3-key management and `--secrets-auth=browser` will be blocked
- Authenticated to staging: `CRE_CLI_ENV=STAGING cre login`

## Verification commands

```bash
cre version                              # must be v1.12.0+
CRE_CLI_ENV=STAGING cre whoami           # verify authenticated to staging
CRE_CLI_ENV=STAGING cre registry list   # confirm private registry visible
```

## Required env vars

| Variable | Required | Notes |
|----------|----------|-------|
| `CRE_CLI_ENV` | Yes | Must be `STAGING` |
| `CRE_API_KEY` | Yes (or browser login) | Auth for staging |

## Intentionally NOT set for positive-path secrets cases

| Variable | Reason |
|----------|--------|
| `CRE_ETH_PRIVATE_KEY` | Off-chain secrets do not require a web3 key |
| `ETH_PRIVATE_KEY` | Same |

## secret.yaml template

```yaml
secretsNames:
  API_KEY:
    - API_KEY_VALUE
```

Export the value before creating:
```bash
export API_KEY_VALUE="your-actual-api-key"
CRE_CLI_ENV=STAGING cre secrets create secret.yaml --secrets-auth=browser
```

## Known issues

- **DEVSVCS-4808**: `ETH_PRIVATE_KEY` is still required even when `--secrets-auth=browser` is passed. Secrets cases are expected to produce partial success until this is resolved.

## Useful links

- Staging UI: https://staging-cre-ui-cll.vercel.app/
- Secrets usage docs: https://docs.chain.link/cre/guides/workflow/secrets/using-secrets-deployed
- Grafana staging logs: https://grafana.ops.prod.cldev.sh (requires VPN)
