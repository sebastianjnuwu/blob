import crypto from "node:crypto";
import "dotenv/config";

type SignInput = {
    id: string;
    exp: number;
    nonce: string;
    method?: string;
};

type VerifyInput = {
    id: string;
    exp: number;
    nonce: string;
    sig: string;
    method?: string;
};

const SIGNING_VERSION = "v1";
const NONCE_PATTERN = /^[A-Za-z0-9_-]{20,64}$/;
const SIGNATURE_PATTERN = /^[A-Za-z0-9_-]{40,256}$/;

function getSecret(): string {
    const secret =
        process.env.TOKEN_SECRET ||
        process.env.SECRET ||
        (process.env.NODE_ENV === "production"
            ? undefined
            : "local-dev-token-secret");

    if (!secret) {
        throw new Error("TOKEN_SECRET is required");
    }

    if (process.env.NODE_ENV === "production" && secret.length < 32) {
        throw new Error(
            "TOKEN_SECRET must have at least 32 characters in production",
        );
    }

    return secret;
}

function buildPayload({ method, id, exp, nonce }: SignInput): string {
    return `${SIGNING_VERSION}:${String(method || "GET").toUpperCase()}:${id}:${exp}:${nonce}`;
}

/**
 * Generates a URL-safe nonce for one-time signed URL payloads.
 */
export function createNonce(): string {
    return crypto.randomBytes(18).toString("base64url");
}

/**
 * Builds an URL-safe signature bound to method, id, expiry and nonce.
 */
export function sign({ id, exp, nonce, method = "GET" }: SignInput): string {
    const payload = buildPayload({ method, id, exp, nonce });

    return crypto
        .createHmac("sha512", getSecret())
        .update(payload)
        .digest("base64url");
}

/**
 * Validates signature structure, expiration and timing-safe signature value.
 */
export function verifySignature({
    id,
    exp,
    nonce,
    sig,
    method = "GET",
}: VerifyInput): boolean {
    if (!id || !exp || !sig || !nonce) {
        return false;
    }

    if (!NONCE_PATTERN.test(String(nonce))) {
        return false;
    }

    if (!SIGNATURE_PATTERN.test(String(sig))) {
        return false;
    }

    if (!Number.isFinite(exp)) {
        return false;
    }

    if (!Number.isSafeInteger(exp)) {
        return false;
    }

    if (Math.floor(Date.now() / 1000) > exp) {
        return false;
    }

    const expectedSignature = sign({ id, exp, nonce, method });
    const expected = Buffer.from(expectedSignature);
    const received = Buffer.from(String(sig));

    if (expected.length !== received.length) {
        return false;
    }

    return crypto.timingSafeEqual(expected, received);
}
