import { createNonce, sign, verifySignature } from "#functions/signer";
import {
  deleteBlobById,
  findBlobById,
  incrementBlobDownloadCount,
  listBlobItems,
  saveBlob,
} from "#services/blob.service";

function asJsonSafeBlob(blob) {
  return blob;
}

const rateLimitStore = new Map();

function consumeRateLimit({ scope, key, max, windowMs }) {
  const now = Date.now();
  const bucketKey = `${scope}:${key}`;
  const current = rateLimitStore.get(bucketKey);

  if (!current || current.resetAt <= now) {
    rateLimitStore.set(bucketKey, {
      count: 1,
      resetAt: now + windowMs,
    });
    return { ok: true, remaining: max - 1 };
  }

  if (current.count >= max) {
    return { ok: false, remaining: 0, retryAfterMs: current.resetAt - now };
  }

  current.count += 1;
  return { ok: true, remaining: Math.max(0, max - current.count) };
}

function getClientIp(req) {
  return req.ip || req.headers["x-forwarded-for"] || "unknown";
}

function getEnvInt(name, fallback) {
  const parsed = Number(process.env[name] ?? fallback);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function parseBoolean(value, fallback = false) {
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

export async function uploadBlob(req, res, next) {
  try {
    if (!req.file) {
      return res
        .status(400)
        .json({ error: "Missing file field in multipart body" });
    }

    const blob = await saveBlob(req.file, {
      bucket: req.body.bucket,
      key: req.body.key,
      isPublic: parseBoolean(req.body.public),
      metadata: req.body.metadata,
    });

    return res.status(201).json(asJsonSafeBlob(blob));
  } catch (error) {
    return next(error);
  }
}

export async function listBlobs(req, res, next) {
  try {
    const page = Number(req.query.page ?? 1);
    const pageSize = Number(req.query.pageSize ?? 20);
    const bucket = req.query.bucket;

    const { data, total } = await listBlobItems({
      page,
      pageSize,
      bucket,
    });

    return res.json({
      page,
      pageSize,
      total,
      items: data.map(asJsonSafeBlob),
    });
  } catch (error) {
    return next(error);
  }
}

export async function getBlob(req, res, next) {
  try {
    const downloadLimit = consumeRateLimit({
      scope: "download",
      key: getClientIp(req),
      max: getEnvInt("DOWNLOAD_RATE_LIMIT_MAX", 120),
      windowMs: getEnvInt("DOWNLOAD_RATE_LIMIT_WINDOW_MS", 60_000),
    });

    if (!downloadLimit.ok) {
      return res.status(429).json({
        error: "Too many requests",
        retryAfterMs: downloadLimit.retryAfterMs,
      });
    }

    const blob = await findBlobById(req.params.id);

    if (!blob) {
      return res.status(404).json({ error: "Blob not found" });
    }

    if (!blob.public) {
      const exp = Number(req.query.exp);
      const sig = req.query.sig;
      const nonce = req.query.n;

      if (!req.query.exp || !req.query.sig || !req.query.n) {
        return res.status(403).json({
          error:
            "Private blob requires signed URL. Use GET /blob/:id/sign first.",
        });
      }

      if (
        !verifySignature({
          id: blob.id,
          exp,
          nonce,
          sig,
          method: req.method,
        })
      ) {
        return res.status(403).json({ error: "Invalid or expired signature" });
      }
    }

    incrementBlobDownloadCount(blob.id).catch(() => {});

    res.setHeader("Content-Type", blob.mime || "application/octet-stream");
    res.setHeader("Content-Disposition", `inline; filename="${blob.filename}"`);
    return res.sendFile(blob.path, { root: process.cwd() });
  } catch (error) {
    return next(error);
  }
}

export async function destroyBlob(req, res, next) {
  try {
    const deleted = await deleteBlobById(req.params.id);

    if (!deleted) {
      return res.status(404).json({ error: "Blob not found" });
    }

    return res.status(204).send();
  } catch (error) {
    return next(error);
  }
}

export async function getBlobSignedUrl(req, res, next) {
  try {
    const signLimit = consumeRateLimit({
      scope: "sign",
      key: `${getClientIp(req)}:${req.params.id}`,
      max: getEnvInt("SIGN_RATE_LIMIT_MAX", 20),
      windowMs: getEnvInt("SIGN_RATE_LIMIT_WINDOW_MS", 60_000),
    });

    if (!signLimit.ok) {
      return res.status(429).json({
        error: "Too many signature requests",
        retryAfterMs: signLimit.retryAfterMs,
      });
    }

    const blob = await findBlobById(req.params.id);

    if (!blob) {
      return res.status(404).json({ error: "Blob not found" });
    }

    const requestedTtl = Number(req.query.ttl ?? 300);
    const minTtl = getEnvInt("SIGNED_URL_TTL_MIN_SECONDS", 30);
    const maxTtl = getEnvInt("SIGNED_URL_TTL_MAX_SECONDS", 900);
    const ttlInSeconds = Math.min(maxTtl, Math.max(minTtl, requestedTtl));
    const exp = Math.floor(Date.now() / 1000) + ttlInSeconds;
    const nonce = createNonce();
    const sig = sign({
      id: blob.id,
      exp,
      nonce,
      method: "GET",
    });

    return res.json({
      id: blob.id,
      exp,
      n: nonce,
      sig,
      ttl: ttlInSeconds,
      url: `/blob/${blob.id}?exp=${exp}&n=${nonce}&sig=${sig}`,
    });
  } catch (error) {
    return next(error);
  }
}
