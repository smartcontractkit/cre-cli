import { CreWorkflowHost } from "./cre-host.js";
import { bindEnvSelector, getSelectedEnvName } from "./lib/cre-env.js";

const terminal = document.getElementById("terminal");
const simulateBtn = document.getElementById("simulate-btn");
const statusEl = document.getElementById("status");
const wasmLink = document.getElementById("wasm-link");
const manifestLink = document.getElementById("manifest-link");
const simJsonLink = document.getElementById("sim-json-link");
const fetchSimBtn = document.getElementById("fetch-sim-btn");

/** @type {{ manifest: object, wasmBytes: ArrayBuffer } | null} */
let prelinked = null;

/** @type {object | null} */
let lastHeadlessJson = null;

/** @type {string | null} */
let lastSimBlobUrl = null;

/** @type {string[]} */
const terminalLines = [];

/** @param {string} line @param {"dim" | "ok" | "err"} [kind] */
function log(line, kind = "dim") {
    terminalLines.push(line);
    const row = document.createElement("div");
    row.className = "line line-" + kind;
    row.textContent = line;
    terminal.appendChild(row);
    terminal.scrollTop = terminal.scrollHeight;
}

function setStatus(text, busy = false) {
    statusEl.textContent = text;
    simulateBtn.disabled = busy || !prelinked;
}

function revokeSimBlob() {
    if (lastSimBlobUrl) {
        URL.revokeObjectURL(lastSimBlobUrl);
        lastSimBlobUrl = null;
    }
}

function publishHeadlessJson(doc) {
    lastHeadlessJson = doc;
    revokeSimBlob();
    const json = JSON.stringify(doc, null, 2);
    const blob = new Blob([json], { type: "application/json" });
    lastSimBlobUrl = URL.createObjectURL(blob);
    simJsonLink.href = lastSimBlobUrl;
    simJsonLink.download = "headless-simulation.json";
    simJsonLink.classList.remove("disabled");
    simJsonLink.removeAttribute("aria-disabled");
    fetchSimBtn.disabled = false;
}

function buildHeadlessDocument(manifest, wasmBytes, report) {
    return {
        mode: "browser-headless",
        generatedAt: new Date().toISOString(),
        workflowWasm: manifest.workflowWasm,
        wasmBytes: wasmBytes.byteLength,
        wasmUrl: manifest.workflowUrl || "./assets/" + manifest.workflowWasm,
        manifestUrl: manifest.manifestUrl || "./assets/manifest.json",
        workflowSimulationResult: report.trigger.workflowSimulationResult ?? null,
        phases: {
            subscribe: report.subscribe,
            trigger: report.trigger,
        },
        terminalLog: [...terminalLines],
    };
}

async function loadManifest() {
    const url = "./assets/manifest.json";
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) {
        throw new Error("Missing " + url);
    }
    return res.json();
}

async function loadWorkflowWasm(manifest) {
    const url = manifest.workflowUrl || "./assets/" + manifest.workflowWasm;
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) {
        throw new Error("Missing workflow WASM at " + url);
    }
    return res.arrayBuffer();
}

async function preloadAssets() {
    setStatus("Loading prelinked WASM…", true);
    const manifest = await loadManifest();
    const wasmBytes = await loadWorkflowWasm(manifest);
    const expected = manifest.wasmBytes;
    if (typeof expected === "number" && expected > 0 && wasmBytes.byteLength !== expected) {
        throw new Error(
            "WASM size mismatch: expected " + expected + " bytes, got " + wasmBytes.byteLength,
        );
    }
    prelinked = { manifest, wasmBytes };

    const wasmUrl = manifest.workflowUrl || "./assets/" + manifest.workflowWasm;
    wasmLink.href = wasmUrl;
    wasmLink.download = manifest.workflowWasm;
    manifestLink.href = manifest.manifestUrl || "./assets/manifest.json";

    const mb = (wasmBytes.byteLength / (1024 * 1024)).toFixed(1);
    setStatus("Ready — " + getSelectedEnvName() + " — " + manifest.workflowWasm + " (" + mb + " MB)");
}

simulateBtn.addEventListener("click", async () => {
    terminal.textContent = "";
    terminalLines.length = 0;
    setStatus("Simulating…", true);
    simulateBtn.disabled = true;

    try {
        if (!prelinked) {
            await preloadAssets();
        }
        const { manifest, wasmBytes } = prelinked;
        log("=== cre workflow simulate (browser headless) ===", "dim");
        log("Guest: " + manifest.workflowWasm + " (" + wasmBytes.byteLength + " bytes)", "dim");
        log("", "dim");

        const host = new CreWorkflowHost(log);
        const report = await host.simulate(wasmBytes, manifest);
        const headless = buildHeadlessDocument(manifest, wasmBytes, report);
        publishHeadlessJson(headless);

        if (report.trigger.workflowSimulationResult) {
            setStatus("Simulation complete");
        } else {
            setStatus("Simulation finished (see terminal)");
        }
    } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        log("Workflow execution failed: " + msg, "err");
        setStatus("Simulation failed");
        console.error(e);
    } finally {
        if (prelinked) {
            simulateBtn.disabled = false;
        }
    }
});

fetchSimBtn.addEventListener("click", async () => {
    if (!lastSimBlobUrl) {
        log("No simulation JSON yet — click Simulate first", "err");
        return;
    }
    try {
        const res = await fetch(lastSimBlobUrl);
        const json = await res.json();
        log("--- fetched headless-simulation.json ---", "dim");
        log(JSON.stringify(json, null, 2), "ok");
        setStatus("Fetched simulation JSON into terminal");
    } catch (e) {
        const msg = e instanceof Error ? e.message : String(e);
        log("Fetch simulation JSON failed: " + msg, "err");
    }
});

simulateBtn.disabled = true;
fetchSimBtn.disabled = true;
simJsonLink.classList.add("disabled");
simJsonLink.setAttribute("aria-disabled", "true");

const envSelect = document.getElementById("cre-env-select");
if (envSelect) {
    bindEnvSelector(envSelect).catch((e) => console.error(e));
}

preloadAssets().catch((e) => {
    const msg = e instanceof Error ? e.message : String(e);
    setStatus("Failed to load WASM");
    log("Could not preload workflow WASM: " + msg, "err");
    console.error(e);
});
