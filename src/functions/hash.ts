import crypto from "node:crypto";

/**
 * Computes a stable content fingerprint.
 */
export function hashContent(buffer: Buffer): string {
    try {
        return crypto.createHash("blake2b512").update(buffer).digest("hex");
    } catch {
        return crypto.createHash("sha512").update(buffer).digest("hex");
    }
}

/**
 * Backward-compatible alias for content hashing.
 */
export function sha256(buffer: Buffer): string {
    return hashContent(buffer);
}
