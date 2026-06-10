const STORAGE_KEY = "cre-demo-auth";

/** @typedef {{ accessToken: string, refreshToken?: string, idToken?: string, expiresAt?: number, apiKey?: string }} AuthState */

export { loadEnv, getSelectedEnvName, setSelectedEnvName, CRE_ENV_STORAGE_KEY } from "./cre-env.js";

/** @returns {AuthState | null} */
export function loadAuth() {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        return raw ? JSON.parse(raw) : null;
    } catch {
        return null;
    }
}

/** @param {AuthState | null} state */
export function saveAuth(state) {
    if (!state) {
        localStorage.removeItem(STORAGE_KEY);
        return;
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

export function clearAuth() {
    saveAuth(null);
}

function b64Url(bytes) {
    const bin = String.fromCharCode(...bytes);
    return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export async function generatePkce() {
    const verifierBytes = crypto.getRandomValues(new Uint8Array(32));
    const verifier = b64Url(verifierBytes);
    const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
    const challenge = b64Url(new Uint8Array(digest));
    return { verifier, challenge };
}

export function randomState() {
    return b64Url(crypto.getRandomValues(new Uint8Array(16)));
}

/**
 * @param {object} env
 * @param {string} challenge
 * @param {string} state
 */
export function buildAuthorizeUrl(env, challenge, state) {
    const params = new URLSearchParams({
        client_id: env.clientId,
        redirect_uri: env.authRedirectUri,
        response_type: "code",
        scope: "openid profile email offline_access",
        code_challenge: challenge,
        code_challenge_method: "S256",
        state,
    });
    if (env.audience) {
        params.set("audience", env.audience);
    }
    return env.authBase + "/authorize?" + params.toString();
}

/**
 * @param {object} env
 * @param {string} code
 * @param {string} verifier
 */
export async function exchangeCode(env, code, verifier) {
    const body = new URLSearchParams({
        grant_type: "authorization_code",
        client_id: env.clientId,
        code,
        redirect_uri: env.authRedirectUri,
        code_verifier: verifier,
    });
    const tokenUrl = env.authTokenUrl || "/cre-auth/oauth/token";
    const res = await fetch(tokenUrl, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body,
    });
    const text = await res.text();
    if (!res.ok) {
        const preview = text.trim().startsWith("<") ? "HTML error page from server" : text.slice(0, 300);
        throw new Error("Token exchange failed (" + res.status + "): " + preview);
    }
    let data;
    try {
        data = JSON.parse(text);
    } catch {
        throw new Error("Token exchange returned non-JSON: " + text.slice(0, 200));
    }
    const expiresAt = data.expires_in ? Date.now() + data.expires_in * 1000 : undefined;
    return {
        accessToken: data.access_token,
        refreshToken: data.refresh_token,
        idToken: data.id_token,
        expiresAt,
    };
}

/**
 * @param {object} env
 * @returns {Promise<AuthState>}
 */
export function loginWithBrowser(env) {
    return new Promise((resolve, reject) => {
        generatePkce()
            .then(async ({ verifier, challenge }) => {
                const state = randomState();
                const url = buildAuthorizeUrl(env, challenge, state);
                sessionStorage.setItem("cre-oauth-verifier", verifier);
                sessionStorage.setItem("cre-oauth-state", state);

                const onMessage = async (ev) => {
                    if (ev.origin !== window.location.origin && ev.origin !== "http://localhost:53682") {
                        return;
                    }
                    const data = ev.data;
                    if (!data || data.type !== "cre-oauth-callback") {
                        return;
                    }
                    window.removeEventListener("message", onMessage);
                    if (data.error) {
                        reject(new Error(data.errorDescription || data.error));
                        return;
                    }
                    const expectedState = sessionStorage.getItem("cre-oauth-state");
                    if (data.state && expectedState && data.state !== expectedState) {
                        reject(new Error("OAuth state mismatch"));
                        return;
                    }
                    const v = sessionStorage.getItem("cre-oauth-verifier");
                    if (!v) {
                        reject(new Error("Missing PKCE verifier"));
                        return;
                    }
                    try {
                        const tokens = await exchangeCode(env, data.code, v);
                        saveAuth(tokens);
                        resolve(tokens);
                    } catch (e) {
                        reject(e);
                    }
                };
                window.addEventListener("message", onMessage);

                const popup = window.open(url, "cre-login", "width=520,height=720");
                if (!popup) {
                    window.removeEventListener("message", onMessage);
                    reject(new Error("Popup blocked — allow popups for this site"));
                    return;
                }
            })
            .catch(reject);
    });
}

/** @param {string} apiKey */
export function saveApiKey(apiKey) {
    saveAuth({ accessToken: "", apiKey: apiKey.trim() });
}

/** @param {AuthState | null} auth */
export function authHeaders(auth) {
    if (!auth) {
        return {};
    }
    if (auth.apiKey) {
        return { Authorization: "Apikey " + auth.apiKey };
    }
    if (auth.accessToken) {
        return { Authorization: "Bearer " + auth.accessToken };
    }
    return {};
}

/** @param {AuthState | null} auth */
export function decodeDeployAccess(auth) {
    if (!auth?.accessToken || auth.apiKey) {
        return auth?.apiKey ? { hasAccess: true, status: "API_KEY" } : null;
    }
    try {
        const payload = JSON.parse(atob(auth.accessToken.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
        let status = "";
        for (const [k, v] of Object.entries(payload)) {
            if (k.endsWith("organization_status") && typeof v === "string") {
                status = v;
            }
        }
        return { hasAccess: status === "FULL_ACCESS", status: status || "unknown" };
    } catch {
        return null;
    }
}
