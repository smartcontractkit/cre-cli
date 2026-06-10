import { authHeaders, loadAuth } from "./cre-auth.js";

function newIdempotencyKey() {
    if (typeof crypto !== "undefined" && crypto.randomUUID) {
        return crypto.randomUUID();
    }
    return "demo-" + Date.now() + "-" + Math.random().toString(36).slice(2);
}

function isMutation(query) {
    return /^\s*mutation\b/i.test(query);
}

/**
 * @param {Response} res
 */
async function readJsonOrThrow(res, label) {
    const text = await res.text();
    const trimmed = text.trim();
    if (trimmed.startsWith("<")) {
        throw new Error(
            label +
                " HTTP " +
                res.status +
                ": server returned HTML (proxy/upstream error). STAGING GraphQL may require VPN.",
        );
    }
    try {
        return JSON.parse(text);
    } catch {
        throw new Error(label + " HTTP " + res.status + ": " + trimmed.slice(0, 300));
    }
}

/**
 * @param {object} env
 * @param {string} query
 * @param {Record<string, unknown>} [variables]
 * @param {{ idempotency?: boolean }} [options]
 */
export async function gql(env, query, variables, options = {}) {
    const auth = loadAuth();
    if (!auth?.accessToken && !auth?.apiKey) {
        throw new Error("Not logged in — use Login or paste an API key");
    }
    const headers = {
        "Content-Type": "application/json",
        "User-Agent": "cre-cli",
        ...authHeaders(auth),
    };
    if (options.idempotency ?? isMutation(query)) {
        headers["Idempotency-Key"] = newIdempotencyKey();
    }
    const res = await fetch(env.graphqlUrl, {
        method: "POST",
        headers,
        body: JSON.stringify({ query, variables }),
    });
    const json = await readJsonOrThrow(res, "GraphQL");
    if (!res.ok) {
        throw new Error("GraphQL HTTP " + res.status + ": " + JSON.stringify(json).slice(0, 400));
    }
    if (json.errors?.length) {
        throw new Error(json.errors.map((e) => e.message).join("; "));
    }
    return json.data;
}

export async function whoami(env) {
    const auth = loadAuth();
    const withEmail = !auth?.apiKey;
    const query = withEmail
        ? `query GetWhoamiDetails {
        getAccountDetails { emailAddress }
        getOrganization { displayName organizationId }
      }`
        : `query GetWhoamiDetails {
        getOrganization { displayName organizationId }
      }`;
    return gql(env, query);
}
