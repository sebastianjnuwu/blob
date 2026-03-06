import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import prisma from "#db/prisma";
import { hashContent } from "#functions/hash";
import { buildStoragePath } from "#functions/storagePath";

const BUCKET_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}$/;

function asHttpError(message, statusCode) {
  const error = new Error(message);
  error.statusCode = statusCode;
  return error;
}

function normalizeBucket(value) {
  const bucket = (value || "default").trim();

  if (!BUCKET_PATTERN.test(bucket)) {
    throw asHttpError("Invalid bucket name", 400);
  }

  return bucket;
}

function normalizeKey(value, originalname) {
  const key = (value || originalname).trim();

  if (!key) {
    throw asHttpError("Missing file key", 400);
  }

  if (key.includes("..") || path.isAbsolute(key) || key.includes("\\")) {
    throw asHttpError("Invalid file key", 400);
  }

  return key;
}

function parseMetadata(value) {
  if (!value) {
    return null;
  }

  if (typeof value === "object") {
    return value;
  }

  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      throw asHttpError("metadata must be a valid JSON object", 400);
    }
  }

  throw asHttpError("metadata must be a valid JSON object", 400);
}

function ensureAllowedMime(mime) {
  const allowedMimes = process.env.ALLOWED_MIME_TYPES?.split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  if (!allowedMimes?.length) {
    return;
  }

  if (!allowedMimes.includes(mime)) {
    throw asHttpError("MIME type not allowed", 415);
  }
}

function createBlobRecord(file, options, hash, relativePath) {
  return {
    id: crypto.randomUUID(),
    bucket: normalizeBucket(options.bucket),
    key: normalizeKey(options.key, file.originalname),
    filename: file.originalname,
    mime: file.mimetype,
    size: Number(file.size),
    hash,
    path: relativePath,
    public: Boolean(options.isPublic),
    version: 1,
    metadata: options.metadata,
  };
}

function deserializeBlob(blob) {
  return {
    ...blob,
    metadata: blob.metadata ? JSON.parse(blob.metadata) : null,
  };
}

export async function saveBlob(file, options = {}) {
  if (!file?.buffer) {
    throw asHttpError("Invalid upload payload", 400);
  }

  ensureAllowedMime(file.mimetype);

  const hash = hashContent(file.buffer);
  const relativePath = buildStoragePath(hash);
  const absolutePath = path.resolve(process.cwd(), relativePath);

  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
  await fs.writeFile(absolutePath, file.buffer, { flag: "w" });

  const existingActive = await prisma.blob.findFirst({
    where: {
      hash,
      deletedAt: null,
    },
  });

  if (existingActive) {
    return deserializeBlob(existingActive);
  }

  const metadata = parseMetadata(options.metadata);
  const record = createBlobRecord(
    file,
    { ...options, metadata: metadata ? JSON.stringify(metadata) : null },
    hash,
    relativePath,
  );

  try {
    const created = await prisma.blob.create({
      data: record,
    });

    return deserializeBlob(created);
  } catch (error) {
    if (error?.code === "P2002") {
      const duplicated = await prisma.blob.findFirst({
        where: {
          hash,
          deletedAt: null,
        },
      });

      if (duplicated) {
        return deserializeBlob(duplicated);
      }
    }

    throw error;
  }
}

export async function findBlobById(id) {
  if (!id) {
    return null;
  }

  const blob = await prisma.blob.findFirst({
    where: {
      id,
      deletedAt: null,
    },
  });

  return blob ? deserializeBlob(blob) : null;
}

export async function listBlobItems({ page = 1, pageSize = 20, bucket } = {}) {
  const safePage = Number.isNaN(page) || page < 1 ? 1 : page;
  const safePageSize =
    Number.isNaN(pageSize) || pageSize < 1 ? 20 : Math.min(pageSize, 100);

  const filterBucket = bucket ? normalizeBucket(bucket) : undefined;
  const where = {
    deletedAt: null,
    ...(filterBucket ? { bucket: filterBucket } : {}),
  };

  const [data, total] = await Promise.all([
    prisma.blob.findMany({
      where,
      orderBy: {
        createdAt: "desc",
      },
      skip: (safePage - 1) * safePageSize,
      take: safePageSize,
    }),
    prisma.blob.count({ where }),
  ]);

  return {
    data: data.map(deserializeBlob),
    total,
  };
}

export async function deleteBlobById(id) {
  const item = await prisma.blob.findFirst({
    where: {
      id,
      deletedAt: null,
    },
  });

  if (!item) {
    return null;
  }

  await prisma.blob.update({
    where: {
      id,
    },
    data: {
      deletedAt: new Date(),
    },
  });

  const absolutePath = path.resolve(process.cwd(), item.path);
  await fs.unlink(absolutePath).catch(() => {});

  return deserializeBlob(item);
}

export async function incrementBlobDownloadCount(id) {
  if (!id) {
    return;
  }

  await prisma.blob.update({
    where: {
      id,
    },
    data: {
      downloads: {
        increment: 1,
      },
    },
  });
}
