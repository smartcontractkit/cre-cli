# Playwright CLI Setup

## Installation

The `playwright-cli` tool is provided by the `@playwright/cli` npm package. The legacy `playwright-cli` npm package is deprecated and should not be used.

### Prerequisites

- Node.js 18+ and npm
- A Chromium-based browser (installed automatically by Playwright on first run)

### Install globally (recommended)

```bash
npm install -g @playwright/cli@latest
```

### Verify installation

```bash
playwright-cli --version
playwright-cli --help
```

If the global binary is not on your PATH, use `npx` as a fallback:

```bash
npx @playwright/cli --version
npx @playwright/cli open https://example.com
```

### Install Playwright browsers

On first use, Playwright may need to download browser binaries. If `open` fails with a missing-browser error:

```bash
npx playwright install chromium
```

## CRE Login Automation

The primary use case for `playwright-cli` in this repo is automating the `cre login` OAuth browser flow so that expect scripts and TUI tests can run without manual intervention.

### Flow overview

1. Start `cre login` in the background — it prints an Auth0 authorization URL and waits.
2. Use `playwright-cli` to open a browser, navigate to the URL, and complete the login form.
3. Auth0 redirects to the CLI's localhost callback, completing the OAuth exchange.
4. `cre login` writes credentials to `~/.cre/cre.yaml` and exits.

### Environment variables

Set these in your `.env` file (copy from `.env.example`):

| Variable | Purpose |
|---|---|
| `CRE_USER_NAME` | Email for CRE login (Auth0) |
| `CRE_PASSWORD` | Password for CRE login (Auth0) |

Do not commit `.env` — it is gitignored.

### Step-by-step: manual playwright-cli auth

```bash
# 1. Start cre login in background, capture the auth URL
./cre login &
CRE_PID=$!
sleep 2

# 2. Extract the authorization URL from cre login output
# (The CLI prints a URL like https://smartcontractkit.eu.auth0.com/authorize?...)

# 3. Open the browser and navigate to the URL
playwright-cli open "$AUTH_URL"

# 4. Take a snapshot to identify form elements
playwright-cli snapshot

# 5. Fill in credentials and submit
playwright-cli fill <email-ref> "$CRE_USER_NAME"
playwright-cli click <continue-ref>
playwright-cli fill <password-ref> "$CRE_PASSWORD"
playwright-cli click <login-ref>

# 6. Wait for redirect to complete, then close browser
sleep 3
playwright-cli close

# 7. Verify login
./cre whoami
```

Element refs (e.g., `<email-ref>`) are obtained from `playwright-cli snapshot` output. The Auth0 login page typically uses:
- An email input field
- A "Continue" button
- A password input field
- A "Log In" / "Continue" button

### Step-by-step: agent-automated auth

When running inside Cursor or another AI coding agent, use the `browser-use` subagent or call `playwright-cli` commands from the shell:

```bash
# Load env vars
source .env

# Start cre login, extract URL
./cre login 2>&1 &
sleep 2

# Agent uses playwright-cli commands to fill forms
playwright-cli open "<auth-url>"
playwright-cli snapshot
# ... fill and click based on snapshot refs ...
playwright-cli close
```

### Verifying credentials after login

```bash
./cre whoami
# Should show Email, Organization ID, Organization Name
```

### Troubleshooting

| Symptom | Fix |
|---|---|
| `playwright-cli: command not found` | Run `npm install -g @playwright/cli@latest` |
| Browser fails to open | Run `npx playwright install chromium` |
| Auth0 shows "Wrong email or password" | Verify `CRE_USER_NAME` and `CRE_PASSWORD` in `.env` |
| `cre login` hangs after browser closes | The redirect may not have hit localhost. Re-run `cre login` and retry. |
| Timeout waiting for auth | Ensure no firewall blocks localhost:8019 (the CLI's callback port) |

## Security Notes

- Never print raw credentials in logs or agent output.
- Report only `set`/`unset` status for environment variables.
- The `.env` file is gitignored; never commit it.
- After login, credentials are stored in `~/.cre/cre.yaml` — protect this file.
