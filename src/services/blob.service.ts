import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import type { Blob as PrismaBlob } from "@prisma/client";
import prisma from "#db/prisma";
import { hashContent } from "#functions/hash";
import { createHttpError } from "#functions/httpError";
import { buildStoragePath } from "#functions/storagePath";

export type BlobResponse = Omit<PrismaBlob, "metadata"> & {
  metadata: Record<string, unknown> | null;
};

type SaveBlobOptions = {
  bucket?: string;
  key?: string;
  isPublic?: boolean;
  metadata?: unknown;
};

type ListBlobParams = {
  page?: number;
  pageSize?: number;
  bucket?: string;
  isPublic?: boolean;
};

const BUCKET_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]{1,62}$/;

// Service layer: persistence + filesystem operations.
function getStorageBasePath(): string {
  return process.env.STORAGE_PATH || "data/blob-storage";
}

const storageBaseAbsolutePath = path.resolve(
  process.cwd(),
  getStorageBasePath(),
);

/**
 * Validates and normalizes bucket input.
 *
 * @param value Raw bucket name.
 * @returns Normalized bucket.
 * @throws {HttpError} When bucket format is invalid.
 */
function normalizeBucket(value?: string): string {
  const bucket = (value || "default").trim();

  if (!BUCKET_PATTERN.test(bucket)) {
    throw createHttpError("Invalid bucket name", 400);
  }

  return bucket;
}

/**
 * Validates and normalizes object key.
 *
 * @param value Requested object key.
 * @param originalname Original uploaded filename.
 * @returns Sanitized key.
 * @throws {HttpError} When key is empty or unsafe.
 */
function normalizeKey(value: string | undefined, originalname: string): string {
  const key = (value || originalname).trim();

  if (!key) {
    throw createHttpError("Missing file key", 400);
  }

  if (key.includes("..") || path.isAbsolute(key) || key.includes("\\")) {
    throw createHttpError("Invalid file key", 400);
  }

  return key;
}

/**
 * Parses metadata payload from multipart field.
 *
 * @param value Metadata field as string/object/undefined.
 * @returns Parsed metadata object or `null`.
 * @throws {HttpError} When metadata is not valid JSON object.
 */
function parseMetadata(value: unknown): Record<string, unknown> | null {
  if (!value) {
    return null;
  }

  if (typeof value === "object") {
    return value as Record<string, unknown>;
  }

  if (typeof value === "string") {
    try {
      return JSON.parse(value) as Record<string, unknown>;
    } catch {
      throw createHttpError("metadata must be a valid JSON object", 400);
    }
  }

  throw createHttpError("metadata must be a valid JSON object", 400);
}

/**
 * Enforces MIME allow-list when configured.
 *
 * @param mime Uploaded MIME type.
 * @throws {HttpError} When MIME is disallowed.
 */
function ensureAllowedMime(mime: string): void {
  const allowedMimes = process.env.ALLOWED_MIME_TYPES?.split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  if (!allowedMimes?.length) {
    return;
  }

  if (!allowedMimes.includes(mime)) {
    throw createHttpError("MIME type not allowed", 415);
  }
}

/**
 * Writes the binary object file to storage path.
 */
async function persistObjectFile(
  absolutePath: string,
  buffer: Buffer,
): Promise<void> {
  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
  await fs.writeFile(absolutePath, buffer, { flag: "w" });
}

function createBlobRecord(
  file: Express.Multer.File,
  options: SaveBlobOptions,
  hash: string,
  relativePath: string,
): Omit<PrismaBlob, "createdAt" | "updatedAt" | "deletedAt" | "downloads"> {
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
    metadata: (options.metadata as string | null) ?? null,
  };
}

// SQLite keeps metadata as string; API returns parsed object.
function deserializeBlob(blob: PrismaBlob): BlobResponse {
  let parsedMetadata: Record<string, unknown> | null = null;

  if (blob.metadata) {
    try {
      parsedMetadata = JSON.parse(blob.metadata) as Record<string, unknown>;
    } catch {
      parsedMetadata = null;
    }
  }

  return {
    ...blob,
    metadata: parsedMetadata,
  };
}

/**
 * Stores a blob file and metadata.
 *
 * Behavior:
 * - Rejects invalid payload/MIME.
 * - Deduplicates by content hash.
 * - Self-heals missing object file when metadata already exists.
 *
 * @param file Parsed multer file payload.
 * @param options Optional upload fields.
 * @returns Persisted blob response.
 * @throws {HttpError} For validation errors.
 */
export async function saveBlob(
  file: Express.Multer.File,
  options: SaveBlobOptions = {},
): Promise<BlobResponse> {
  if (!file?.buffer) {
    throw createHttpError("Invalid upload payload", 400);
  }

  ensureAllowedMime(file.mimetype);

  const hash = hashContent(file.buffer);
  const relativePath = buildStoragePath(hash);
  const absolutePath = path.resolve(process.cwd(), relativePath);

  const existingActive = await prisma.blob.findFirst({
    where: {
      hash,
      deletedAt: null,
    },
  });

  if (existingActive) {
    // Self-heal file if metadata exists but object file is missing.
    await fs.access(absolutePath).catch(async () => {
      await persistObjectFile(absolutePath, file.buffer);
    });

    return deserializeBlob(existingActive);
  }

  await persistObjectFile(absolutePath, file.buffer);

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
    const prismaError = error as { code?: string };

    if (prismaError?.code === "P2002") {
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

/**
 * Finds an active blob by ID.
 */
export async function findBlobById(id: string): Promise<BlobResponse | null> {
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

/**
 * Returns paginated blob metadata.
 */
export async function listBlobItems({
  page = 1,
  pageSize = 20,
  bucket,
  isPublic,
}: ListBlobParams = {}): Promise<{ data: BlobResponse[]; total: number }> {
  const safePage = Number.isNaN(page) || page < 1 ? 1 : page;
  const safePageSize =
    Number.isNaN(pageSize) || pageSize < 1 ? 20 : Math.min(pageSize, 100);

  const filterBucket = bucket ? normalizeBucket(bucket) : undefined;
  const where = {
    deletedAt: null,
    ...(filterBucket ? { bucket: filterBucket } : {}),
    ...(isPublic === undefined ? {} : { public: isPublic }),
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

/**
 * Soft-deletes a blob and attempts to remove its file.
 */
export async function deleteBlobById(id: string): Promise<BlobResponse | null> {
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

/**
 * Increments download counter for observability.
 */
export async function incrementBlobDownloadCount(id: string): Promise<void> {
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

/**
 * Resolves a blob path and guarantees it is inside storage root.
 */
export function resolveBlobAbsolutePath(blobPath: string): string {
  const absolutePath = path.resolve(process.cwd(), blobPath);
  const relativeToStorage = path.relative(
    storageBaseAbsolutePath,
    absolutePath,
  );

  if (
    relativeToStorage.startsWith("..") ||
    path.isAbsolute(relativeToStorage)
  ) {
    // Hard fail if DB path points outside storage root.
    throw createHttpError("Invalid blob path", 400);
  }

  return absolutePath;
}
