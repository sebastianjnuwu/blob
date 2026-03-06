import { sign, verifySignature } from "#functions/signer";
import {
  deleteBlobById,
  findBlobById,
  listBlobItems,
  saveBlob,
} from "#services/blob.service";

function asJsonSafeBlob(blob) {
  return {
    ...blob,
    size: Number(blob.size),
  };
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
    const blob = await findBlobById(req.params.id);

    if (!blob) {
      return res.status(404).json({ error: "Blob not found" });
    }

    if (!blob.public) {
      const exp = Number(req.query.exp);
      const sig = req.query.sig;

      if (!verifySignature({ id: blob.id, exp, sig })) {
        return res.status(403).json({ error: "Invalid or expired signature" });
      }
    }

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
    const blob = await findBlobById(req.params.id);

    if (!blob) {
      return res.status(404).json({ error: "Blob not found" });
    }

    const ttlInSeconds = Number(req.query.ttl ?? 300);
    const exp = Math.floor(Date.now() / 1000) + ttlInSeconds;
    const sig = sign(blob.id, exp);

    return res.json({
      id: blob.id,
      exp,
      sig,
      url: `/blob/${blob.id}?exp=${exp}&sig=${sig}`,
    });
  } catch (error) {
    return next(error);
  }
}
