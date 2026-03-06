import crypto from "node:crypto";

/**
 * Computes a stable content fingerprint for deduplication.
 *
 * Uses `blake2b512` when available and falls back to `sha512`
 * to keep compatibility across different OpenSSL builds.
 */
export function hashContent(buffer: Buffer): string {
  try {
    return crypto.createHash("blake2b512").update(buffer).digest("hex");
  } catch {
    return crypto.createHash("sha512").update(buffer).digest("hex");
  }
}
