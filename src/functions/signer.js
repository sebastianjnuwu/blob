import crypto from "node:crypto";
import "dotenv/config";

function getSecret() {
    const secret = process.env.TOKEN_SECRET || process.env.SECRET;

    if (!secret) {
        throw new Error("TOKEN_SECRET is required");
    }

    return secret;
}

export function sign(id, exp) {
    const payload = `${id}:${exp}`;

    return crypto.createHmac("sha256", getSecret()).update(payload).digest("hex");
}

export function verifySignature({ id, exp, sig }) {
    if (!id || !exp || !sig) {
        return false;
    }

    if (!Number.isFinite(exp)) {
        return false;
    }

    if (Math.floor(Date.now() / 1000) > exp) {
        return false;
    }

    const expectedSignature = sign(id, exp);
    const expected = Buffer.from(expectedSignature);
    const received = Buffer.from(String(sig));

    if (expected.length !== received.length) {
        return false;
    }

    return crypto.timingSafeEqual(expected, received);
}
