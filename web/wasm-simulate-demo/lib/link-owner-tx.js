import { Interface } from "https://esm.sh/ethers@6.13.4";

const linkOwnerIface = new Interface([
    "function linkOwner(uint256 validityTimestamp, bytes32 proof, bytes signature)",
]);

/**
 * Build LinkOwner calldata like cre-cli cmd/account/link_key (EOA path).
 * @param {object} linking initiateLinking response
 */
export function encodeLinkOwnerCalldata(linking) {
    const expiresAt = Date.parse(linking.validUntil);
    if (Number.isNaN(expiresAt)) {
        throw new Error("invalid validUntil: " + linking.validUntil);
    }
    const validityTimestamp = BigInt(Math.floor(expiresAt / 1000));

    let proof = linking.ownershipProofHash;
    if (!proof) {
        throw new Error("missing ownershipProofHash");
    }
    if (!proof.startsWith("0x")) {
        proof = "0x" + proof;
    }

    let signature = linking.signature;
    if (!signature) {
        throw new Error("missing signature from initiateLinking");
    }
    if (!signature.startsWith("0x")) {
        signature = "0x" + signature;
    }

    return linkOwnerIface.encodeFunctionData("linkOwner", [validityTimestamp, proof, signature]);
}

/**
 * @param {object} linking
 * @returns {string} hex calldata
 */
export function resolveLinkTransactionData(linking) {
    const raw = linking.transactionData;
    if (raw && raw !== "0x" && raw !== "0X") {
        return raw.startsWith("0x") ? raw : "0x" + raw;
    }
    return encodeLinkOwnerCalldata(linking);
}
