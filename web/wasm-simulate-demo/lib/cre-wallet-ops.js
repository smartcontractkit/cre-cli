import { gql } from "./cre-graphql.js";
import { resolveLinkTransactionData } from "./link-owner-tx.js";
import { ensureChain, sendTransaction } from "./metamask.js";

const LINK_ENV = "PRODUCTION_TESTNET";

/**
 * @param {object} env
 * @param {string} ownerAddress
 * @param {string} ownerLabel
 */
export async function initiateLinking(env, ownerAddress, ownerLabel) {
    const mutation = `
mutation InitiateLinking($request: InitiateLinkingRequest!) {
  initiateLinking(request: $request) {
    ownershipProofHash
    workflowOwnerAddress
    validUntil
    signature
    chainSelector
    contractAddress
    transactionData
    functionSignature
    functionArgs
  }
}`;
    const data = await gql(env, mutation, {
        request: {
            workflowOwnerAddress: ownerAddress,
            workflowOwnerLabel: ownerLabel,
            environment: LINK_ENV,
            requestProcess: "EOA",
        },
    });
    return data.initiateLinking;
}

/**
 * Submit link-owner tx via MetaMask using server-prepared calldata.
 * @param {object} linking
 * @param {object} env
 */
export async function registerWalletOnChain(linking, env) {
    const chainId = env.workflowRegistryChainIdHex || "0x1";
    await ensureChain(chainId);
    const to = linking.contractAddress || env.workflowRegistryAddress;
    const data = resolveLinkTransactionData(linking);
    const hash = await sendTransaction({ to, data, chainId });
    return { hash, explorer: env.workflowRegistryChainExplorerUrl + "/tx/" + hash };
}

/**
 * Demo publish: whoami + optional offchain upsert when binary URL is known.
 * @param {object} env
 * @param {string} owner
 * @param {{ workflowId?: string, binaryUrl?: string, workflowName?: string, tag?: string }} opts
 */
export async function publishWorkflowDemo(env, owner, opts) {
    const steps = [];
    const who = await gql(
        env,
        `query { getOrganization { displayName organizationId } }`,
    );
    steps.push("Organization: " + who.getOrganization.displayName);

    if (!opts.binaryUrl) {
        steps.push("Publish: upload WASM via cre workflow deploy (or set binary URL after upload).");
        steps.push("Browser demo does not yet compute workflow ID — use CLI deploy with MetaMask txs copied from Register wallet flow.");
        return { steps, upserted: false };
    }

    const mutation = `
mutation UpsertOffchainWorkflow($request: UpsertOffchainWorkflowRequest!) {
  upsertOffchainWorkflow(request: $request) {
    workflow { workflowId workflowName status binaryUrl configUrl owner }
  }
}`;
    const workflow = {
        workflowId: opts.workflowId,
        workflowName: opts.workflowName || env.demoWorkflowName,
        status: "ACTIVE",
        binaryUrl: opts.binaryUrl,
        donFamily: env.donFamily,
        owner,
    };
    if (opts.tag) {
        workflow.tag = opts.tag;
    }
    const result = await gql(env, mutation, { request: { workflow } });
    steps.push("Upserted offchain workflow: " + result.upsertOffchainWorkflow.workflow.workflowId);
    return { steps, upserted: true, workflow: result.upsertOffchainWorkflow.workflow };
}
