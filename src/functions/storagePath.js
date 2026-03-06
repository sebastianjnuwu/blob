import path from "node:path";

export function buildStoragePath(hash) {
  const a = hash.slice(0, 2);
  const b = hash.slice(2, 4);
  const base = process.env.STORAGE_PATH || "data/blob-storage";

  return path.join(base, "objects", a, b, hash);
}
