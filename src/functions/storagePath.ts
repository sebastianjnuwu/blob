import path from "node:path";

const HASH_HEX_PATTERN = /^[a-f0-9]{32,128}$/i;

/**
 * Computes a deterministic storage path from a content hash.
 *
 * @throws {Error} When hash format is invalid.
 */
export function buildStoragePath(hash: string): string {
  if (!HASH_HEX_PATTERN.test(hash)) {
    throw new Error("Invalid blob hash format");
  }

  const a = hash.slice(0, 2);
  const b = hash.slice(2, 4);
  const base = process.env.STORAGE_PATH || "data/blob-storage";

  return path.join(base, "objects", a, b, hash);
}
