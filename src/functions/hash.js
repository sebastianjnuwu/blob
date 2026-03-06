import crypto from "node:crypto";

export function hashContent(buffer) {
  try {
    return crypto.createHash("blake2b512").update(buffer).digest("hex");
  } catch {
    return crypto.createHash("sha512").update(buffer).digest("hex");
  }
}

export function sha256(buffer) {
  return hashContent(buffer);
}
