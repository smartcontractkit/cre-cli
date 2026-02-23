# Setup

## Required tools

- `go`
- `script` (or equivalent PTY-capable terminal tool)
- `expect` (for deterministic local replay scripts)
- `bun`
- `node` (or `nvm` + selected node version)
- `forge`
- `anvil`
- `playwright-cli` (for browser automation flows)

## Optional tools

- `npx` fallback for Playwright CLI if global binary is unavailable

## Install hints

### macOS (Homebrew)

```bash
brew install expect bun foundry
foundryup || true
```

For Node via nvm:

```bash
export NVM_DIR="$HOME/.nvm"
. "$NVM_DIR/nvm.sh"
nvm use 22
```

### Linux (apt + foundry)

```bash
sudo apt-get update
sudo apt-get install -y expect curl build-essential
curl -fsSL https://bun.sh/install | bash
curl -L https://foundry.paradigm.xyz | bash
foundryup
```

Install Node via nvm as needed.

### Windows

- PTY semantics differ. Prefer Linux/macOS for deterministic expect-based interactive tests.
- Use script/non-interactive checks on Windows where possible.

## Environment variables by scenario

- Browser auth automation:
  - `CRE_USER_NAME`
  - `CRE_PASSWORD`
- API-key auth path:
  - `CRE_API_KEY`
- Simulation/on-chain path (testnet only):
  - `CRE_ETH_PRIVATE_KEY`

## Verification commands

```bash
command -v go script expect bun node forge anvil playwright-cli

go version
bun --version
node -v
forge --version
anvil --version
playwright-cli --version
```

## Security

- Do not print actual secret values.
- Report only `set`/`unset` status for env variables.
