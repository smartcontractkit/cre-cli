import {
    clearAuth,
    decodeDeployAccess,
    loadAuth,
    loadEnv,
    loginWithBrowser,
    saveApiKey,
} from "./lib/cre-auth.js";
import { bindEnvSelector } from "./lib/cre-env.js";
import { whoami } from "./lib/cre-graphql.js";
import { initiateLinking, publishWorkflowDemo, registerWalletOnChain } from "./lib/cre-wallet-ops.js";
import {
    MetaMaskUserRejectedError,
    connectWallet,
    formatMetaMaskError,
    getChainIdHex,
    getConnectedAddress,
    hasMetaMask,
    installGlobalSigner,
    onMetaMaskStateChange,
    shortAddress,
} from "./lib/metamask.js";

const terminal = document.getElementById("terminal");
const statusEl = document.getElementById("wallet-status");
const mmIndicator = document.getElementById("mm-indicator");
const mmIndicatorLabel = document.getElementById("mm-indicator-label");
const loginBtn = document.getElementById("login-btn");
const registerBtn = document.getElementById("register-wallet-btn");
const publishBtn = document.getElementById("publish-wf-btn");
const connectBtn = document.getElementById("connect-mm-btn");
const apiKeyInput = document.getElementById("api-key-input");
const apiKeySaveBtn = document.getElementById("api-key-save-btn");
const logoutBtn = document.getElementById("logout-btn");
const ownerLabelInput = document.getElementById("owner-label-input");

/** @type {object | null} */
let env = null;

const CHAIN_NAMES = {
    "0x1": "Mainnet",
    "0xaa36a7": "Sepolia",
};

/** @param {string} line @param {"dim" | "ok" | "err"} [kind] */
function log(line, kind = "dim") {
    const row = document.createElement("div");
    row.className = "line line-" + kind;
    row.textContent = line;
    terminal.appendChild(row);
    terminal.scrollTop = terminal.scrollHeight;
}

/** @param {unknown} err */
function logMetaMaskError(err, context) {
    const msg = formatMetaMaskError(err);
    if (err instanceof MetaMaskUserRejectedError || (typeof err === "object" && err && "code" in err && err.code === 4001)) {
        log((context ? context + ": " : "") + msg, "err");
        return;
    }
    log((context ? context + ": " : "") + msg, "err");
}

function setStatus(text) {
    statusEl.textContent = text;
}

/**
 * @param {{ address: string | null, chainId: string | null }} state
 */
async function updateMetaMaskIndicator(state = {}) {
    if (!mmIndicator || !mmIndicatorLabel) {
        return;
    }
    const address = state.address !== undefined ? state.address : await getConnectedAddress();
    const chainId =
        state.chainId !== undefined ? state.chainId : env ? await getChainIdHex() : await getChainIdHex();

    mmIndicator.classList.remove("mm-on", "mm-off", "mm-warn");
    connectBtn?.classList.remove("mm-connected");

    if (!hasMetaMask()) {
        mmIndicator.classList.add("mm-off");
        mmIndicatorLabel.textContent = "MetaMask not installed";
        if (connectBtn) {
            connectBtn.textContent = "Connect MetaMask";
            connectBtn.disabled = true;
        }
        return;
    }

    if (connectBtn) {
        connectBtn.disabled = false;
    }

    if (!address) {
        mmIndicator.classList.add("mm-off");
        mmIndicatorLabel.textContent = "MetaMask not connected";
        if (connectBtn) {
            connectBtn.textContent = "Connect MetaMask";
        }
        return;
    }

    const expectedChain = env?.workflowRegistryChainIdHex?.toLowerCase();
    const actualChain = chainId?.toLowerCase();
    const chainLabel = CHAIN_NAMES[actualChain ?? ""] ?? actualChain ?? "unknown network";
    const onExpectedChain = !expectedChain || actualChain === expectedChain;

    if (onExpectedChain) {
        mmIndicator.classList.add("mm-on");
        mmIndicatorLabel.textContent = "Connected · " + shortAddress(address) + " · " + chainLabel;
        if (connectBtn) {
            connectBtn.textContent = "Connected";
            connectBtn.classList.add("mm-connected");
        }
    } else {
        mmIndicator.classList.add("mm-warn");
        const expectedLabel = CHAIN_NAMES[expectedChain ?? ""] ?? expectedChain;
        mmIndicatorLabel.textContent =
            "Connected · " + shortAddress(address) + " · wrong network (" + chainLabel + ", need " + expectedLabel + ")";
        if (connectBtn) {
            connectBtn.textContent = "Switch network";
        }
    }
}

function refreshButtons() {
    const auth = loadAuth();
    const mm = hasMetaMask();
    const loggedIn = !!(auth?.accessToken || auth?.apiKey);
    registerBtn.disabled = !loggedIn || !mm;
    publishBtn.disabled = !loggedIn;
    loginBtn.disabled = false;
}

async function refreshHeader() {
    const auth = loadAuth();
    const addr = await getConnectedAddress();
    const parts = [];
    if (auth?.apiKey) {
        parts.push("API key");
    } else if (auth?.accessToken) {
        parts.push("Logged in");
    } else {
        parts.push("Not logged in");
    }
    if (addr) {
        parts.push("MM " + shortAddress(addr));
    }
    setStatus(parts.join(" · "));
    await updateMetaMaskIndicator({ address: addr });
    refreshButtons();
}

async function init() {
    const envSelect = document.getElementById("cre-env-select");
    if (envSelect) {
        await bindEnvSelector(envSelect);
    }
    log("=== CRE account & wallet (browser) ===", "dim");
    env = await loadEnv();
    log("Environment: " + env.envName, "dim");
    log("Registry: " + env.workflowRegistryAddress + " (" + env.workflowRegistryChainName + ")", "dim");
    if (!hasMetaMask()) {
        log("MetaMask not detected — install extension for on-chain signing.", "err");
    } else {
        installGlobalSigner();
        log("MetaMask signer registered for simulate page (window.__creMetaMaskSigner).", "ok");
        onMetaMaskStateChange((state) => {
            updateMetaMaskIndicator(state);
            refreshButtons();
        });
    }
    await refreshHeader();
}

loginBtn.addEventListener("click", async () => {
    setStatus("Opening login…");
    log("Opening CRE login (same as cre login)…", "dim");
    try {
        await loginWithBrowser(env);
        log("Login successful — tokens saved.", "ok");
        const access = decodeDeployAccess(loadAuth());
        if (access) {
            log(
                "Deploy access: " +
                    (access.hasAccess ? "enabled (" + access.status + ")" : "not enabled (" + access.status + ")"),
                access.hasAccess ? "ok" : "dim",
            );
        }
        try {
            const data = await whoami(env);
            const org = data.getOrganization;
            if (data.getAccountDetails?.emailAddress) {
                log("Email: " + data.getAccountDetails.emailAddress, "dim");
            }
            log("Org: " + org.displayName + " (" + org.organizationId + ")", "ok");
        } catch (e) {
            const msg = e instanceof Error ? e.message : String(e);
            log("Account details (GraphQL) failed: " + msg, "err");
            if (env.envName === "STAGING" || env.envName === "DEVELOPMENT") {
                log("STAGING/DEV GraphQL is on Tailscale — connect VPN, or switch CRE_CLI_ENV to PRODUCTION.", "dim");
            }
        }
    } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        log("Login failed: " + msg, "err");
    }
    await refreshHeader();
});

connectBtn.addEventListener("click", async () => {
    try {
        const addr = await connectWallet();
        installGlobalSigner();
        log("MetaMask connected: " + addr, "ok");
        await updateMetaMaskIndicator({ address: addr });
    } catch (e) {
        logMetaMaskError(e, "MetaMask connect");
    }
    await refreshHeader();
});

registerBtn.addEventListener("click", async () => {
    try {
        const owner = await connectWallet();
        await updateMetaMaskIndicator({ address: owner });
        const label = ownerLabelInput.value.trim() || "browser-demo";
        log("Initiating link-key for " + owner + " …", "dim");
        const linking = await initiateLinking(env, owner, label);
        log("initiateLinking OK — contract " + linking.contractAddress, "ok");
        log("Opening MetaMask to sign linkOwner transaction…", "dim");
        const { hash, explorer } = await registerWalletOnChain(linking, env);
        log("Transaction sent: " + hash, "ok");
        log("Explorer: " + explorer, "dim");
        log("Same flow as: cre account link-key", "dim");
    } catch (e) {
        logMetaMaskError(e, "Register wallet");
    }
    await refreshHeader();
});

publishBtn.addEventListener("click", async () => {
    log("Publish workflow (demo)…", "dim");
    try {
        let owner = await getConnectedAddress();
        if (!owner) {
            owner = await connectWallet();
        }
        const result = await publishWorkflowDemo(env, owner, {
            workflowName: env.demoWorkflowName,
            tag: env.demoWorkflowTag,
        });
        for (const line of result.steps) {
            log(line, result.upserted ? "ok" : "dim");
        }
        log("Full on-chain deploy: use cre workflow deploy with MetaMask for each tx (no private key in .env).", "dim");
    } catch (e) {
        logMetaMaskError(e, "Publish workflow");
    }
});

apiKeySaveBtn.addEventListener("click", async () => {
    const key = apiKeyInput.value.trim();
    if (!key) {
        log("Enter an API key first.", "err");
        return;
    }
    saveApiKey(key);
    log("API key saved locally.", "ok");
    await refreshHeader();
});

logoutBtn.addEventListener("click", async () => {
    clearAuth();
    apiKeyInput.value = "";
    log("Logged out.", "dim");
    await refreshHeader();
});

init().catch((e) => {
    logMetaMaskError(e, "Init");
    setStatus("Init failed");
});
