/** @typedef {{ workflowWasm: string, subscribeB64: string, triggerB64: string, cronTypeUrl: string }} DemoManifest */

/**
 * @param {number} fieldNum
 * @param {Uint8Array} data
 */
function tagBytes(fieldNum, data) {
    const tag = (fieldNum << 3) | 2;
    const out = [];
    out.push(...encodeVarint(tag));
    out.push(...encodeVarint(data.length));
    out.push(...data);
    return new Uint8Array(out);
}

/** @param {number} n */
function encodeVarint(n) {
    const out = [];
    let v = n >>> 0;
    while (v >= 0x80) {
        out.push((v & 0x7f) | 0x80);
        v >>>= 7;
    }
    out.push(v);
    return out;
}

/** @param {number} fieldNum @param {number} n */
function tagVarint(fieldNum, n) {
    const tag = (fieldNum << 3) | 0;
    return new Uint8Array([...encodeVarint(tag), ...encodeVarint(n)]);
}

/** @param {Date} date */
function encodeTimestamp(date) {
    const ms = date.getTime();
    const seconds = Math.floor(ms / 1000);
    const nanos = (ms % 1000) * 1_000_000;
    const parts = [tagVarint(1, seconds)];
    if (nanos !== 0) {
        parts.push(tagVarint(2, nanos));
    }
    return concatBytes(parts);
}

/** @param {string} typeUrl @param {Uint8Array} value */
function encodeAny(typeUrl, value) {
    const enc = new TextEncoder();
    const urlBytes = enc.encode(typeUrl);
    const parts = [tagBytes(1, urlBytes), tagBytes(2, value)];
    return concatBytes(parts);
}

/** @param {Date} date */
function encodeCronPayload(date) {
    return tagBytes(1, encodeTimestamp(date));
}

/** @param {Date} scheduled @param {string} cronTypeUrl */
export function buildTriggerExecuteRequest(scheduled, cronTypeUrl) {
    const cronPayload = encodeCronPayload(scheduled);
    const triggerMsg = concatBytes([
        tagVarint(1, 0),
        tagBytes(2, encodeAny(cronTypeUrl, cronPayload)),
    ]);
    const config = new TextEncoder().encode("{}");
    return concatBytes([
        tagBytes(1, config),
        tagBytes(3, triggerMsg),
        tagVarint(4, 2048),
    ]);
}

/** @param {DemoManifest} manifest */
export function buildSubscribeExecuteRequest(manifest) {
    const raw = atob(manifest.subscribeB64);
    return new Uint8Array([...raw].map((c) => c.charCodeAt(0)));
}

/** @param {Uint8Array} bytes */
export function toWasmArgv(bytes) {
    const b64 = btoa(String.fromCharCode(...bytes));
    return ["wasm", b64];
}

/** @param {Uint8Array[]} parts */
function concatBytes(parts) {
    const total = parts.reduce((n, p) => n + p.length, 0);
    const out = new Uint8Array(total);
    let off = 0;
    for (const p of parts) {
        out.set(p, off);
        off += p.length;
    }
    return out;
}
