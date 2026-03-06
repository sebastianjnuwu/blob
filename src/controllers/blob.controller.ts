import fs from "node:fs/promises";
import path from "node:path";
import type { NextFunction, Request, Response } from "express";
import { createHttpError } from "#functions/httpError";
import type { RateLimitResult } from "#functions/rateLimit";
import { consumeRateLimit } from "#functions/rateLimit";
import { createNonce, sign, verifySignature } from "#functions/signer";
import {
    deleteBlobById,
    findBlobById,
    incrementBlobDownloadCount,
    listBlobItems,
    resolveBlobAbsolutePath,
    saveBlob,
} from "#services/blob.service";

// Controller layer: HTTP concerns only (input parsing, rate limits, status codes).

/**
 * Parses a positive integer from env with fallback.
 */
function getEnvInt(name: string, fallback: number): number {
    const parsed = Number(process.env[name] ?? fallback);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

const CONFIG = {
    signRateLimitMax: getEnvInt("SIGN_RATE_LIMIT_MAX", 20),
    signRateLimitWindowMs: getEnvInt("SIGN_RATE_LIMIT_WINDOW_MS", 60_000),
    downloadRateLimitMax: getEnvInt("DOWNLOAD_RATE_LIMIT_MAX", 120),
    downloadRateLimitWindowMs: getEnvInt("DOWNLOAD_RATE_LIMIT_WINDOW_MS", 60_000),
    signedUrlTtlMinSeconds: getEnvInt("SIGNED_URL_TTL_MIN_SECONDS", 30),
    signedUrlTtlMaxSeconds: getEnvInt("SIGNED_URL_TTL_MAX_SECONDS", 900),
};

/**
 * Extracts best-effort client IP for rate limiting.
 */
function getClientIp(req: Request): string {
    return req.ip || String(req.headers["x-forwarded-for"] || "unknown");
}

/**
 * Adds rate limit metadata headers to the response.
 */
function setRateLimitHeaders(res: Response, rateLimit: RateLimitResult): void {
    res.setHeader("x-ratelimit-remaining", String(rateLimit.remaining));
    res.setHeader(
        "x-ratelimit-reset",
        String(Math.floor(rateLimit.resetAt / 1000)),
    );
}

/**
 * Converts common truthy/falsy payloads to boolean.
 */
function parseBoolean(value: unknown, fallback = false): boolean {
    if (value === undefined) {
        return fallback;
    }

    if (typeof value === "boolean") {
        return value;
    }

    if (typeof value === "string") {
        return value.toLowerCase() === "true";
    }

    return fallback;
}

/**
 * Parses optional boolean query params (`true|false`).
 *
 * @throws {HttpError} When query value is present but invalid.
 */
function parseOptionalBooleanQuery(value: unknown): boolean | undefined {
    if (value === undefined) {
        return undefined;
    }

    if (typeof value === "boolean") {
        return value;
    }

    if (typeof value !== "string") {
        throw createHttpError("Invalid boolean query value", 400);
    }

    const normalized = value.trim().toLowerCase();
    if (normalized === "true") {
        return true;
    }
    if (normalized === "false") {
        return false;
    }

    throw createHttpError("Invalid boolean query value. Use true or false", 400);
}

/**
 * Handles multipart upload and persists blob metadata/content.
 *
 * Expected payload (`multipart/form-data`):
 * - `file` (required)
 * - `bucket` (optional)
 * - `key` (optional)
 * - `public` (optional, `true|false`)
 * - `metadata` (optional, JSON string)
 */
export async function uploadBlob(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        if (!req.file) {
            res.status(400).json({ error: "Missing file field in multipart body" });
            return;
        }

        const blob = await saveBlob(req.file, {
            bucket: req.body.bucket,
            key: req.body.key,
            isPublic: parseBoolean(req.body.public),
            metadata: req.body.metadata,
        });

        res.status(201).json(blob);
        return;
    } catch (error) {
        next(error);
        return;
    }
}

/**
 * Returns paginated blob metadata list.
 *
 * Query params:
 * - `page` (default: 1)
 * - `pageSize` (default: 20, max: 100)
 * - `bucket` (optional)
 * - `public` (optional, `true|false`)
 */
export async function listBlobs(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        const page = Number(req.query.page ?? 1);
        const pageSize = Number(req.query.pageSize ?? 20);
        const bucket =
            typeof req.query.bucket === "string" ? req.query.bucket : undefined;
        const isPublic = parseOptionalBooleanQuery(req.query.public);

        const { data, total } = await listBlobItems({
            page,
            pageSize,
            bucket,
            isPublic,
        });

        res.json({
            page,
            pageSize,
            total,
            items: data,
        });
        return;
    } catch (error) {
        next(error);
        return;
    }
}

/**
 * Streams a blob file if access conditions are satisfied.
 *
 * Private blobs require querystring signature fields: `exp`, `n`, `sig`.
 */
export async function getBlob(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        // Apply early throttling to reduce expensive work under abuse.
        const downloadLimit = consumeRateLimit({
            scope: "download",
            key: getClientIp(req),
            max: CONFIG.downloadRateLimitMax,
            windowMs: CONFIG.downloadRateLimitWindowMs,
        });

        setRateLimitHeaders(res, downloadLimit);

        if (!downloadLimit.ok) {
            res.status(429).json({
                error: "Too many requests",
                retryAfterMs: downloadLimit.retryAfterMs,
            });
            return;
        }

        const blobId = String(req.params.id);
        const blob = await findBlobById(blobId);

        if (!blob) {
            res.status(404).json({ error: "Blob not found" });
            return;
        }

        if (!blob.public) {
            const exp = Number(req.query.exp);
            const sig = req.query.sig;
            const nonce = req.query.n;

            if (!req.query.exp || !req.query.sig || !req.query.n) {
                res.status(403).json({
                    error:
                        "Private blob requires signed URL. Use GET /blob/:id/sign first.",
                });
                return;
            }

            if (
                !verifySignature({
                    id: blob.id,
                    exp,
                    nonce: String(nonce),
                    sig: String(sig),
                    method: req.method,
                })
            ) {
                res.status(403).json({ error: "Invalid or expired signature" });
                return;
            }
        }

        // Async metric update; never block file delivery.
        incrementBlobDownloadCount(blob.id).catch(() => { });

        const absolutePath = resolveBlobAbsolutePath(blob.path);
        await fs.access(absolutePath);

        res.setHeader("Content-Type", blob.mime || "application/octet-stream");
        res.setHeader("Content-Disposition", `inline; filename="${blob.filename}"`);
        res.sendFile(path.resolve(absolutePath));
        return;
    } catch (error) {
        next(error);
        return;
    }
}

/**
 * Soft-deletes blob metadata and removes file when present.
 */
export async function destroyBlob(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        const blobId = String(req.params.id);
        const deleted = await deleteBlobById(blobId);

        if (!deleted) {
            res.status(404).json({ error: "Blob not found" });
            return;
        }

        res.status(204).send();
        return;
    } catch (error) {
        next(error);
        return;
    }
}

/**
 * Issues a signed URL payload for private blob download.
 *
 * Query params:
 * - `ttl` in seconds, clamped by environment min/max bounds.
 */
export async function getBlobSignedUrl(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        // Limit signed URL minting to reduce brute-force and scraping pressure.
        const signLimit = consumeRateLimit({
            scope: "sign",
            key: `${getClientIp(req)}:${String(req.params.id)}`,
            max: CONFIG.signRateLimitMax,
            windowMs: CONFIG.signRateLimitWindowMs,
        });

        setRateLimitHeaders(res, signLimit);

        if (!signLimit.ok) {
            res.status(429).json({
                error: "Too many signature requests",
                retryAfterMs: signLimit.retryAfterMs,
            });
            return;
        }

        const blobId = String(req.params.id);
        const blob = await findBlobById(blobId);

        if (!blob) {
            res.status(404).json({ error: "Blob not found" });
            return;
        }

        const requestedTtl = Number(req.query.ttl ?? 300);
        const minTtl = CONFIG.signedUrlTtlMinSeconds;
        const maxTtl = CONFIG.signedUrlTtlMaxSeconds;
        const ttlInSeconds = Math.min(maxTtl, Math.max(minTtl, requestedTtl));
        const exp = Math.floor(Date.now() / 1000) + ttlInSeconds;
        const nonce = createNonce();
        const sig = sign({
            id: blob.id,
            exp,
            nonce,
            method: "GET",
        });

        res.json({
            id: blob.id,
            exp,
            n: nonce,
            sig,
            ttl: ttlInSeconds,
            url: `/blob/${blob.id}?exp=${exp}&n=${nonce}&sig=${sig}`,
        });
        return;
    } catch (error) {
        next(error);
        return;
    }
}
