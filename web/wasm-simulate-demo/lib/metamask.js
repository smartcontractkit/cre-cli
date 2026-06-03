/**
 * In-page MetaMask signer — replaces CRE_ETH_PRIVATE_KEY for browser flows.
 */

const MAINNET_CHAIN_ID = "0x1";

/** User canceled in MetaMask (EIP-1193 4001). */
export class MetaMaskUserRejectedError extends Error {
    /** @param {string} [message] */
    constructor(message = "Transaction canceled in MetaMask (you declined signing).") {
        super(message);
        this.name = "MetaMaskUserRejectedError";
        this.code = 4001;
    }
}

/**
 * Turn MetaMask RPC / provider errors into readable strings (never "[object Object]").
 * @param {unknown} err
 */
export function formatMetaMaskError(err) {
    if (err == null) {
        return "Unknown MetaMask error";
    }
    if (typeof err === "string") {
        return err.replace(/^MetaMask - RPC Error:\s*/i, "").trim();
    }
    if (err instanceof MetaMaskUserRejectedError) {
        return err.message;
    }
    if (err instanceof Error && err.message) {
        return err.message.replace(/^MetaMask - RPC Error:\s*/i, "").trim();
    }

    const obj = /** @type {{ code?: number | string; message?: string; data?: { code?: number; message?: string } }} */ (err);
    const code = obj.code ?? obj.data?.code;
    const message = (obj.message ?? obj.data?.message ?? "").replace(/^MetaMask - RPC Error:\s*/i, "").trim();

    if (code === 4001 || code === "4001") {
        if (/signature|transaction|tx/i.test(message)) {
            return "Transaction canceled in MetaMask (you declined signing).";
        }
        if (/connect|account/i.test(message)) {
            return "Connection canceled in MetaMask.";
        }
        return "Request canceled in MetaMask.";
    }
    if (code === 4100) {
        return "MetaMask: connect your wallet first.";
    }
    if (code === 4902) {
        return "MetaMask: add the required network in your wallet.";
    }
    if (message) {
        return message;
    }
    try {
        return JSON.stringify(err);
    } catch {
        return "MetaMask error (could not parse details)";
    }
}

/** @param {unknown} err */
function rethrowMetaMaskError(err) {
    const message = formatMetaMaskError(err);
    const code =
        typeof err === "object" && err != null && "code" in err
            ? /** @type {{ code?: number }} */ (err).code
            : undefined;
    if (code === 4001) {
        throw new MetaMaskUserRejectedError(message);
    }
    throw new Error(message);
}

/** @returns {import('ethers').Eip1193Provider | null} */
export function getEthereum() {
    const eth = globalThis.ethereum;
    if (!eth || typeof eth.request !== "function") {
        return null;
    }
    return eth;
}

export function hasMetaMask() {
    return getEthereum() != null;
}

/** @param {string} chainIdHex e.g. 0x1 */
export async function getChainIdHex() {
    const eth = getEthereum();
    if (!eth) {
        return null;
    }
    return eth.request({ method: "eth_chainId" });
}

/** @param {string} chainIdHex e.g. 0x1 */
export async function ensureChain(chainIdHex = MAINNET_CHAIN_ID) {
    const eth = getEthereum();
    if (!eth) {
        throw new Error("MetaMask is not installed");
    }
    try {
        const current = await eth.request({ method: "eth_chainId" });
        if (current?.toLowerCase() === chainIdHex.toLowerCase()) {
            return;
        }
        await eth.request({
            method: "wallet_switchEthereumChain",
            params: [{ chainId: chainIdHex }],
        });
    } catch (e) {
        rethrowMetaMaskError(e);
    }
}

export async function connectWallet() {
    const eth = getEthereum();
    if (!eth) {
        throw new Error("MetaMask is not installed. Install the extension and refresh.");
    }
    try {
        const accounts = await eth.request({ method: "eth_requestAccounts" });
        if (!accounts?.length) {
            throw new Error("No account selected in MetaMask");
        }
        return accounts[0];
    } catch (e) {
        rethrowMetaMaskError(e);
    }
}

export async function getConnectedAddress() {
    const eth = getEthereum();
    if (!eth) {
        return null;
    }
    try {
        const accounts = await eth.request({ method: "eth_accounts" });
        return accounts?.[0] ?? null;
    } catch {
        return null;
    }
}

/** Remove in-page signer used by the simulate host. */
export function clearGlobalSigner() {
    delete globalThis.__creMetaMaskSigner;
}

/** Revoke site access to the selected account in MetaMask and clear the in-page signer. */
export async function disconnectWallet() {
    const eth = getEthereum();
    if (eth) {
        try {
            await eth.request({
                method: "wallet_revokePermissions",
                params: [{ eth_accounts: {} }],
            });
        } catch (e) {
            const code =
                typeof e === "object" && e != null && "code" in e
                    ? /** @type {{ code?: number }} */ (e).code
                    : undefined;
            if (code === 4001) {
                rethrowMetaMaskError(e);
            }
            // Unsupported or nothing to revoke — still clear app-side signer below.
        }
    }
    clearGlobalSigner();
}

/** @param {string} address */
export function shortAddress(address) {
    if (!address || address.length < 10) {
        return address ?? "";
    }
    return address.slice(0, 6) + "…" + address.slice(-4);
}

/** @param {string} message */
export async function signMessage(message) {
    const eth = getEthereum();
    if (!eth) {
        throw new Error("MetaMask is not available");
    }
    const from = await connectWallet();
    const hex =
        "0x" +
        [...new TextEncoder().encode(message)]
            .map((b) => b.toString(16).padStart(2, "0"))
            .join("");
    try {
        return await eth.request({
            method: "personal_sign",
            params: [hex, from],
        });
    } catch (e) {
        rethrowMetaMaskError(e);
    }
}

/**
 * @param {{ to: string, data?: string, value?: string, gas?: string, chainId?: string }} tx
 */
export async function sendTransaction(tx) {
    const eth = getEthereum();
    if (!eth) {
        throw new Error("MetaMask is not available");
    }
    const from = await connectWallet();
    if (tx.chainId) {
        await ensureChain(tx.chainId);
    }
    try {
        return await eth.request({
            method: "eth_sendTransaction",
            params: [
                {
                    from,
                    to: tx.to,
                    data: tx.data ?? "0x",
                    value: tx.value ?? "0x0",
                    gas: tx.gas,
                },
            ],
        });
    } catch (e) {
        rethrowMetaMaskError(e);
    }
}

/**
 * @param {(payload: { address: string | null, chainId: string | null }) => void} listener
 */
export function onMetaMaskStateChange(listener) {
    const eth = getEthereum();
    if (!eth?.on) {
        return () => {};
    }
    const refresh = () => {
        Promise.all([getConnectedAddress(), getChainIdHex()])
            .then(([address, chainId]) => listener({ address, chainId }))
            .catch(() => listener({ address: null, chainId: null }));
    };
    eth.on("accountsChanged", refresh);
    eth.on("chainChanged", refresh);
    refresh();
    return () => {
        eth.removeListener?.("accountsChanged", refresh);
        eth.removeListener?.("chainChanged", refresh);
    };
}

/**
 * @returns {{ signMessage: (msg: string) => Promise<string>, sendTransaction: (tx: object) => Promise<string>, getAddress: () => Promise<string> }}
 */
export function createExternalSigner() {
    return {
        async getAddress() {
            const a = await getConnectedAddress();
            if (!a) {
                throw new Error("Connect MetaMask first");
            }
            return a;
        },
        signMessage,
        sendTransaction,
    };
}

/** Install global signer used by simulate host when present. */
export function installGlobalSigner() {
    globalThis.__creMetaMaskSigner = createExternalSigner();
}
