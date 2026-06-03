# Browser CRE workflow simulate demo

Standalone page that runs the **hello-world-go** workflow WASM in the browser. A JavaScript **simulator host** implements the CRE `env` ABI (the same contract `cre workflow simulate` uses to drive guest WASM). No API key, GraphQL, or RPC.

## Docker (recommended)

From the **cre-cli repo root**:

```bash
./web/wasm-simulate-demo/run.sh
# or: docker compose -f web/wasm-simulate-demo/docker-compose.yml up --build -d
```

Open [http://localhost:9090](http://localhost:9090) — the image compiles `hello-world.wasm` and bakes it into `/assets/`. The page **preloads** manifest + WASM on load.

**Account & wallet** ([http://localhost:9090/wallet.html](http://localhost:9090/wallet.html)):

- **Login** — OAuth popup (same redirect as `cre login`, port **53682** via `oauth-callback` service)
- **Connect MetaMask** — use extension instead of `CRE_ETH_PRIVATE_KEY`
- **Register wallet** — `cre account link-key` flow: GraphQL `initiateLinking` + MetaMask transaction
- **Publish workflow** — checks org access; full deploy still uses `cre workflow deploy` for artifact upload / on-chain register (MetaMask for each tx)

OAuth callback must be reachable at `http://localhost:53682/callback` (started automatically by `docker compose`).

**Environment** — toolbar **CRE_CLI_ENV** selector (default **STAGING**, same as `export CRE_CLI_ENV=STAGING` for the CLI). Changing it reloads the page and switches API/auth proxies.

Port override: `WASM_DEMO_PORT=8787 ./web/wasm-simulate-demo/run.sh`

### Page downloads instead of opening in Chrome?

Chrome does that when the server sends `Content-Type: application/octet-stream` for HTML (common if nginx is missing `include mime.types`). Check:

```bash
curl -sI http://localhost:9090/ | grep -i content-type
```

You want `Content-Type: text/html` (one header). If you see `application/octet-stream`, rebuild the demo image (`docker compose ... up --build`) and make sure nothing else is bound to port 8787. Do not open `index.html` via `file://` — use `http://localhost:8787/`.

## Local build (without Docker)

```bash
./scripts/build-wasm-demo.sh
python3 -m http.server 9090 --directory web/wasm-simulate-demo
```

Open [http://localhost:9090](http://localhost:9090)

## Expected output

Click **Run simulation**. The terminal shows subscribe + trigger phases and JSON like:

```json
{
  "Result": "Fired at 2026-06-02T12:34:56.789Z"
}
```

## Architecture

The Docker image runs `tools/build_assets` (Go `GOOS=wasip1` compile) then nginx serves static files with `assets/hello-world.wasm` and `assets/manifest.json` prelinked to `./assets/` URLs in the page.
