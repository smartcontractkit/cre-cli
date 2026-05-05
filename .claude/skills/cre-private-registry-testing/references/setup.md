# Setup

## Prerequisites

- CRE CLI v1.12.0 or later: `cre update` or follow https://docs.chain.link/cre/getting-started/cli-installation
- Tailscale VPN active and connected to @smartcontract.com network
- Access to a staging organization at https://staging-cre-ui-cll.vercel.app/
  - All staging orgs have deploy access by default; create one if needed
  - Contact De Clercq Wentzel or Emmanuel Jacquier for the Vercel password
- Authenticated to staging: `CRE_CLI_ENV=STAGING cre login`

## Verification commands

```bash
cre version                              # must be v1.12.0+
CRE_CLI_ENV=STAGING cre whoami           # verify authenticated to staging
CRE_CLI_ENV=STAGING cre registry list   # must show "private" off-chain registry
```

Expected `registry list` output:
```
Registries available to your organization ethereum-testnet-sepolia (0xaE55...1135)
  ID:   onchain:ethereum-testnet-sepolia
  Type: on-chain
  Addr: 0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135

Private (Chainlink-hosted)
  ID:   private
  Type: off-chain
```

## Required env vars

| Variable | Required | Notes |
|----------|----------|-------|
| `CRE_CLI_ENV` | Yes | Must be `STAGING` |
| `CRE_API_KEY` | Yes (or browser login) | Auth for staging |

## Intentionally NOT set for positive-path cases

| Variable | Reason |
|----------|--------|
| `CRE_ETH_PRIVATE_KEY` | Private registry does not require a web3 key |
| `ETH_PRIVATE_KEY` | Same — must be absent for positive-path tests |

Setting these when deploying to a private registry should produce an error (see test case 3).

## workflow.yaml configuration for private registry

```yaml
deployment-registry: "private"
```

Always use `staging-settings` as the target:

```bash
CRE_CLI_ENV=STAGING cre workflow simulate <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow deploy <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow pause <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow activate <folder> --target=staging-settings
CRE_CLI_ENV=STAGING cre workflow delete <folder> --target=staging-settings
```

## Useful links

- Staging UI: https://staging-cre-ui-cll.vercel.app/
- Grafana staging logs: https://grafana.ops.prod.cldev.sh (requires VPN)
- workflow.yaml registry field docs: https://docs.google.com/document/d/121Kc4pCTjMNaQHhxSUTCTMdlJT0HuacffJqsFNKM2iA
