/** Same as CRE_CLI_ENV / cre-cli internal/environments. */
export const CRE_ENV_STORAGE_KEY = "cre-demo-env";

/** @typedef {keyof typeof PROXY_PREFIX} CreEnvName */

const PROXY_PREFIX = {
    DEVELOPMENT: { api: "/cre-api-dev", auth: "/cre-auth-dev" },
    STAGING: { api: "/cre-api-staging", auth: "/cre-auth-staging" },
    PRODUCTION: { api: "/cre-api", auth: "/cre-auth" },
};

/** @type {{ defaultEnv: string, environments: Record<string, object>, demoWorkflowName: string, demoWorkflowTag: string, authRedirectUri: string } | null} */
let catalog = null;

export async function loadEnvironmentsCatalog() {
    if (catalog) {
        return catalog;
    }
    const res = await fetch("./assets/cre-environments.json", { cache: "no-store" });
    if (!res.ok) {
        throw new Error("Missing assets/cre-environments.json");
    }
    catalog = await res.json();
    return catalog;
}

/** @returns {CreEnvName} */
export function getSelectedEnvName() {
    try {
        const stored = localStorage.getItem(CRE_ENV_STORAGE_KEY);
        if (stored && stored in PROXY_PREFIX) {
            return /** @type {CreEnvName} */ (stored);
        }
    } catch {
        /* ignore */
    }
    return "STAGING";
}

/** @param {CreEnvName} name */
export function setSelectedEnvName(name) {
    if (!(name in PROXY_PREFIX)) {
        throw new Error("Unknown environment: " + name);
    }
    localStorage.setItem(CRE_ENV_STORAGE_KEY, name);
}

/**
 * Resolved config for the active CRE_CLI_ENV (proxied API paths for browser CORS).
 * @returns {Promise<object>}
 */
export async function loadEnv() {
    const cat = await loadEnvironmentsCatalog();
    const envName = getSelectedEnvName();
    const base = cat.environments[envName];
    if (!base) {
        throw new Error("Environment not in catalog: " + envName);
    }
    const proxy = PROXY_PREFIX[envName];
    return {
        ...base,
        envName,
        graphqlUrl: proxy.api + "/graphql",
        authTokenUrl: proxy.auth + "/oauth/token",
        demoWorkflowName: cat.demoWorkflowName,
        demoWorkflowTag: cat.demoWorkflowTag,
        authRedirectUri: cat.authRedirectUri,
    };
}

/** @param {HTMLElement} selectEl */
export async function bindEnvSelector(selectEl) {
    const cat = await loadEnvironmentsCatalog();
    const current = getSelectedEnvName();
    const defaultName = cat.defaultEnv || "STAGING";
    if (!localStorage.getItem(CRE_ENV_STORAGE_KEY)) {
        setSelectedEnvName(
            defaultName in PROXY_PREFIX ? /** @type {CreEnvName} */ (defaultName) : "STAGING",
        );
    }

    selectEl.innerHTML = "";
    for (const name of Object.keys(cat.environments)) {
        const opt = document.createElement("option");
        opt.value = name;
        opt.textContent = name;
        if (name === getSelectedEnvName()) {
            opt.selected = true;
        }
        selectEl.appendChild(opt);
    }

    selectEl.addEventListener("change", () => {
        setSelectedEnvName(/** @type {CreEnvName} */ (selectEl.value));
        window.location.reload();
    });
}
