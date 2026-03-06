import path from "node:path";

/**
 * Computes the deterministic storage path from a blob content hash.
 */
export function buildStoragePath(hash: string): string {
    const a = hash.slice(0, 2);
    const b = hash.slice(2, 4);
    const base = process.env.STORAGE_PATH || "data/blob-storage";

    return path.join(base, "objects", a, b, hash);
}
