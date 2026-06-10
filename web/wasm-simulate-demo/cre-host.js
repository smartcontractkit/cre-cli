import { buildSubscribeExecuteRequest, buildTriggerExecuteRequest, toWasmArgv } from "./payload.js";
import { patchWasiForGoWasip1 } from "./wasi-stubs.js";

/** @typedef {(line: string, kind?: "dim" | "ok" | "err") => void} LogFn */

/**
 * @typedef {object} PhaseResult
 * @property {boolean} ok
 * @property {string} [triggerId]
 * @property {{ Result?: string } | null} [workflowSimulationResult]
 * @property {string | null} [error]
 * @property {string} rawResponseBase64
 */

/**
 * @typedef {object} SimulateReport
 * @property {PhaseResult} subscribe
 * @property {PhaseResult} trigger
 */

/**
 * Browser CRE simulator host: loads workflow WASM and implements the `env` imports
 * used by cre-sdk-go (same contract as `cre workflow simulate`, without API keys).
 */
export class CreWorkflowHost {
    /** @param {LogFn} log */
    constructor(log) {
        this.log = log;
        this.memory = null;
        this.lastResponse = null;
    }

    /**
     * @param {ArrayBuffer} wasmBytes
     * @param {import("./payload.js").DemoManifest} manifest
     * @returns {Promise<SimulateReport>}
     */
    async simulate(wasmBytes, manifest) {
        this.log("Workflow compiled (loaded prelinked WASM)", "dim");
        this.log("[SIMULATION] Simulator Initialized", "ok");

        const subscribeReq = buildSubscribeExecuteRequest(manifest);
        this.log("Registering cron trigger (subscribe phase)...", "dim");
        const subResponse = await this.invokeGuest(wasmBytes, subscribeReq);
        const subscribe = this.parseSubscribePhase(subResponse);

        const triggerReq = buildTriggerExecuteRequest(new Date(), manifest.cronTypeUrl);
        this.log("Firing cron trigger (index 0)...", "dim");
        const execResponse = await this.invokeGuest(wasmBytes, triggerReq);
        const trigger = this.parseTriggerPhase(execResponse);

        this.logExecutionToTerminal(trigger);
        return { subscribe, trigger };
    }

    /** @param {Uint8Array} raw */
    parseSubscribePhase(raw) {
        const b64 = bytesToBase64(raw);
        if (raw.length === 0) {
            this.log("Subscribe returned empty response", "err");
            return { ok: false, error: "empty subscribe response", rawResponseBase64: b64 };
        }
        const text = new TextDecoder().decode(raw);
        if (text.includes("cron-trigger@1.0.0")) {
            this.log("Registered trigger: cron-trigger@1.0.0", "ok");
        } else {
            this.log("Subscribe phase completed", "ok");
        }
        return {
            ok: true,
            triggerId: text.includes("cron-trigger@1.0.0") ? "cron-trigger@1.0.0" : undefined,
            rawResponseBase64: b64,
        };
    }

    /** @param {Uint8Array} raw */
    parseTriggerPhase(raw) {
        const b64 = bytesToBase64(raw);
        const firedAt = extractUtf8String(raw, "Fired at");
        if (firedAt) {
            return {
                ok: true,
                workflowSimulationResult: { Result: firedAt },
                rawResponseBase64: b64,
            };
        }
        const preview = new TextDecoder("utf-8", { fatal: false }).decode(raw.slice(0, 400));
        if (raw.length > 0 && (preview.includes("error") || preview.includes("Error"))) {
            return {
                ok: false,
                error: preview,
                rawResponseBase64: b64,
            };
        }
        return {
            ok: raw.length > 0,
            error: raw.length === 0 ? "empty trigger response" : null,
            rawResponseBase64: b64,
        };
    }

    /** @param {PhaseResult} trigger */
    logExecutionToTerminal(trigger) {
        if (trigger.workflowSimulationResult) {
            this.log("Workflow Simulation Result:", "ok");
            this.log(JSON.stringify(trigger.workflowSimulationResult, null, 2));
            return;
        }
        if (trigger.error) {
            this.log("Execution resulted in an error: " + trigger.error, "err");
            return;
        }
        this.log("Workflow finished without a parsed result", "ok");
    }

    /**
     * @param {ArrayBuffer} wasmBytes
     * @param {Uint8Array} executeRequest
     */
    async invokeGuest(wasmBytes, executeRequest) {
        this.lastResponse = null;
        const argv = toWasmArgv(executeRequest);
        const env = this.buildEnvImports();
        const wasi = new WASI(argv, [], createDefaultWasiFds());
        patchWasiForGoWasip1(wasi);
        wasiHackStdio(wasi, (chunk) => {
            const line = new TextDecoder().decode(chunk).trimEnd();
            if (line) {
                this.log(line, "dim");
            }
        });

        const { instance } = await WebAssembly.instantiate(wasmBytes, {
            env,
            wasi_snapshot_preview1: wasi.wasiImport,
        });

        this.memory = instance.exports.memory;
        try {
            wasi.start(instance);
        } catch (e) {
            const msg = e instanceof Error ? e.message : String(e);
            if (!msg.includes("exit with exit code")) {
                throw e;
            }
        }
        return this.lastResponse ?? new Uint8Array();
    }

    buildEnvImports() {
        const host = this;
        return {
            send_response(ptr, len) {
                const mem = host.memory;
                if (!mem) {
                    return -1;
                }
                host.lastResponse = new Uint8Array(mem.buffer, ptr, len).slice();
                return 0;
            },
            version_v2_go() {},
            switch_modes() {},
            now(ptr) {
                const mem = host.memory;
                if (!mem) {
                    return -1;
                }
                const view = new DataView(mem.buffer);
                const ns = BigInt(Date.now()) * 1_000_000n;
                view.setBigUint64(ptr, ns, true);
                return 0;
            },
            log(ptr, len) {
                const mem = host.memory;
                if (!mem) {
                    return;
                }
                const text = new TextDecoder().decode(new Uint8Array(mem.buffer, ptr, len));
                if (text.trim()) {
                    host.log(text.trimEnd(), "dim");
                }
            },
            call_capability() {
                // When Account & wallet page connected MetaMask, __creMetaMaskSigner is set.
                // Full capability→MetaMask routing for chain-write WASM is not wired yet;
                // use wallet.html for login, link-key, and publish txs.
                if (globalThis.__creMetaMaskSigner) {
                    host.log("[host] MetaMask signer available (connect on Account & wallet page)", "dim");
                }
                return -1n;
            },
            await_capabilities(ptr, len, outPtr, outMax) {
                const mem = host.memory;
                if (!mem) {
                    return -1n;
                }
                const msg = "capability not available in browser demo host";
                const enc = new TextEncoder().encode(msg);
                const n = Math.min(enc.length, outMax);
                new Uint8Array(mem.buffer, outPtr, n).set(enc.subarray(0, n));
                return -BigInt(n);
            },
            get_secrets(_ptr, _len, outPtr, outMax) {
                const mem = host.memory;
                if (!mem) {
                    return -1n;
                }
                const msg = "secrets not available in browser demo host";
                const enc = new TextEncoder().encode(msg);
                const n = Math.min(enc.length, outMax);
                new Uint8Array(mem.buffer, outPtr, n).set(enc.subarray(0, n));
                return -BigInt(n);
            },
            await_secrets(_ptr, _len, outPtr, outMax) {
                const mem = host.memory;
                if (!mem) {
                    return -1n;
                }
                const msg = "secrets not available in browser demo host";
                const enc = new TextEncoder().encode(msg);
                const n = Math.min(enc.length, outMax);
                new Uint8Array(mem.buffer, outPtr, n).set(enc.subarray(0, n));
                return -BigInt(n);
            },
            random_seed() {
                return 42n;
            },
        };
    }
}

/** @returns {unknown[]} fds for browser_wasi_shim (stdin, stdout, stderr, preopened /) */
function createDefaultWasiFds() {
    const { File, OpenFile, PreopenDirectory } = globalThis;
    const empty = () => new File(new ArrayBuffer(0));
    return [
        new OpenFile(empty()),
        new OpenFile(empty()),
        new OpenFile(empty()),
        new PreopenDirectory("/", {}),
    ];
}

/** @param {WASI} wasi @param {(chunk: Uint8Array) => void} onWrite */
function wasiHackStdio(wasi, onWrite) {
    const write = wasi.wasiImport.fd_write;
    wasi.wasiImport.fd_write = (fd, iov, iovcnt, nwritten) => {
        if (fd === 1 || fd === 2) {
            const mem = wasi.inst?.exports?.memory;
            if (mem) {
                const view = new DataView(mem.buffer);
                let total = 0;
                for (let i = 0; i < iovcnt; i++) {
                    const buf = view.getUint32(iov + 8 * i, true);
                    const len = view.getUint32(iov + 8 * i + 4, true);
                    onWrite(new Uint8Array(mem.buffer, buf, len));
                    total += len;
                }
                view.setUint32(nwritten, total, true);
                return 0;
            }
        }
        return write(fd, iov, iovcnt, nwritten);
    };
}

/** @param {Uint8Array} buf @param {string} needle */
function extractUtf8String(buf, needle) {
    const text = new TextDecoder("utf-8", { fatal: false }).decode(buf);
    const idx = text.indexOf(needle);
    if (idx < 0) {
        return null;
    }
    let end = idx + needle.length;
    while (end < text.length && text.charCodeAt(end) >= 32 && text.charCodeAt(end) !== 0) {
        end++;
    }
    return text.slice(idx, end).trim();
}

/** @param {Uint8Array} bytes */
function bytesToBase64(bytes) {
    let binary = "";
    const chunk = 0x8000;
    for (let i = 0; i < bytes.length; i += chunk) {
        binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
    }
    return btoa(binary);
}
