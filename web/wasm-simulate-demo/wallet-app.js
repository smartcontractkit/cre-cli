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
    disconnectWallet,
    ensureChain,
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
const loginBtn = document.getElementById("login-btn");
const registerBtn = document.getElementById("register-wallet-btn");
const publishBtn = document.getElementById("publish-wf-btn");
const connectBtn = document.getElementById("connect-mm-btn");
const switchBtn = document.getElementById("switch-mm-btn");
const walletMmAddress = document.getElementById("wallet-mm-address");
const walletMmStatus = document.getElementById("wallet-mm-status");
const apiKeyInput = document.getElementById("api-key-input");
const apiKeySaveBtn = document.getElementById("api-key-save-btn");
const logoutBtn = document.getElementById("logout-btn");
const ownerLabelInput = document.getElementById("owner-label-input");

/** @type {object | null} */
let env = null;

/** Ignore stale async MetaMask reads when events fire in quick succession. */
let mmSyncGen = 0;

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

/** @param {HTMLElement | null} el @param {boolean} hidden */
function setHidden(el, hidden) {
    if (!el) {
        return;
    }
    el.hidden = hidden;
    el.classList.toggle("is-hidden", hidden);
}

/** @param {string | null} address @param {string | null} chainId */
function renderWalletPanel(address, chainId) {
    const chainLabel = CHAIN_NAMES[chainId?.toLowerCase() ?? ""] ?? chainId ?? "";
    const expectedChain = env?.workflowRegistryChainIdHex?.toLowerCase();
    const actualChain = chainId?.toLowerCase();
    const onExpectedChain = !expectedChain || !actualChain || actualChain === expectedChain;

    if (walletMmAddress) {
        if (!address) {
            walletMmAddress.textContent = "Not connected";
            walletMmAddress.classList.remove("wallet-mm-address-connected");
        } else {
            walletMmAddress.textContent = address + (chainLabel ? " · " + chainLabel : "");
            walletMmAddress.classList.add("wallet-mm-address-connected");
        }
    }

    if (walletMmStatus) {
        walletMmStatus.classList.remove("wallet-mm-status-off", "wallet-mm-status-on", "wallet-mm-status-warn");
        if (!hasMetaMask()) {
            walletMmStatus.textContent = "Extension not installed";
            walletMmStatus.classList.add("wallet-mm-status-off");
        } else if (!address) {
            walletMmStatus.textContent = "Disconnected";
            walletMmStatus.classList.add("wallet-mm-status-off");
        } else if (onExpectedChain) {
            walletMmStatus.textContent = "Connected";
            walletMmStatus.classList.add("wallet-mm-status-on");
        } else {
            const expectedLabel = CHAIN_NAMES[expectedChain ?? ""] ?? expectedChain;
            walletMmStatus.textContent = "Wrong network (need " + expectedLabel + ")";
            walletMmStatus.classList.add("wallet-mm-status-warn");
        }
    }

    if (!hasMetaMask()) {
        if (connectBtn) {
            connectBtn.textContent = "Connect MetaMask";
            setHidden(connectBtn, false);
            connectBtn.disabled = true;
        }
        setHidden(switchBtn, true);
        return;
    }

    if (connectBtn) {
        connectBtn.disabled = false;
        connectBtn.textContent = "Connect MetaMask";
    }

    if (!address) {
        setHidden(connectBtn, false);
        setHidden(switchBtn, true);
        return;
    }

    setHidden(connectBtn, true);
    setHidden(switchBtn, false);
    if (switchBtn) {
        switchBtn.disabled = false;
    }
}

async function syncMetaMaskUI() {
    const gen = ++mmSyncGen;
    const [address, chainId] = await Promise.all([getConnectedAddress(), getChainIdHex()]);
    if (gen !== mmSyncGen) {
        return;
    }
    renderWalletPanel(address, chainId);
    refreshButtons();
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
    await syncMetaMaskUI();
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
        onMetaMaskStateChange(() => {
            void syncMetaMaskUI();
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

connectBtn?.addEventListener("click", async () => {
    try {
        if (env?.workflowRegistryChainIdHex) {
            await ensureChain(env.workflowRegistryChainIdHex);
        }
        const addr = await connectWallet();
        installGlobalSigner();
        log("MetaMask connected: " + addr, "ok");
    } catch (e) {
        logMetaMaskError(e, "MetaMask connect");
    }
    await refreshHeader();
});

switchBtn?.addEventListener("click", async () => {
    const previous = await getConnectedAddress();
    try {
        await disconnectWallet();
        if (env?.workflowRegistryChainIdHex) {
            await ensureChain(env.workflowRegistryChainIdHex);
        }
        const addr = await connectWallet();
        installGlobalSigner();
        if (previous && previous.toLowerCase() !== addr.toLowerCase()) {
            log("MetaMask switched from " + shortAddress(previous) + " to " + addr, "ok");
        } else {
            log("MetaMask wallet: " + addr, "ok");
        }
    } catch (e) {
        logMetaMaskError(e, "Switch wallet");
    }
    await refreshHeader();
});

registerBtn.addEventListener("click", async () => {
    try {
        const owner = await connectWallet();
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
