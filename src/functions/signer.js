import crypto from "node:crypto";
import "dotenv/config";

const SIGNING_VERSION = "v1";

function getSecret() {
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

function buildPayload({ method, id, exp, nonce }) {
    return `${SIGNING_VERSION}:${method}:${id}:${exp}:${nonce}`;
}

export function createNonce() {
    return crypto.randomBytes(18).toString("base64url");
}

export function sign({ id, exp, nonce, method = "GET" }) {
    const payload = buildPayload({ method, id, exp, nonce });

    return crypto
        .createHmac("sha512", getSecret())
        .update(payload)
        .digest("base64url");
}

export function verifySignature({ id, exp, nonce, sig, method = "GET" }) {
    if (!id || !exp || !sig || !nonce) {
        return false;
    }

    if (!Number.isFinite(exp)) {
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
