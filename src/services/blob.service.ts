import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import type { Blob as PrismaBlob } from "@prisma/client";
import prisma from "#db/prisma";
import { hashContent } from "#functions/hash";
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

function asHttpError(
  message: string,
  statusCode: number,
): Error & { statusCode: number } {
  const error = new Error(message) as Error & { statusCode: number };
  error.statusCode = statusCode;
  return error;
}

function normalizeBucket(value?: string): string {
  const bucket = (value || "default").trim();

  if (!BUCKET_PATTERN.test(bucket)) {
    throw asHttpError("Invalid bucket name", 400);
  }

  return bucket;
}

function normalizeKey(value: string | undefined, originalname: string): string {
  const key = (value || originalname).trim();

  if (!key) {
    throw asHttpError("Missing file key", 400);
  }

  if (key.includes("..") || path.isAbsolute(key) || key.includes("\\")) {
    throw asHttpError("Invalid file key", 400);
  }

  return key;
}

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
      throw asHttpError("metadata must be a valid JSON object", 400);
    }
  }

  throw asHttpError("metadata must be a valid JSON object", 400);
}

function ensureAllowedMime(mime: string): void {
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

export async function saveBlob(
  file: Express.Multer.File,
  options: SaveBlobOptions = {},
): Promise<BlobResponse> {
  if (!file?.buffer) {
    throw asHttpError("Invalid upload payload", 400);
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
      await fs.mkdir(path.dirname(absolutePath), { recursive: true });
      await fs.writeFile(absolutePath, file.buffer, { flag: "w" });
    });

    return deserializeBlob(existingActive);
  }

  await fs.mkdir(path.dirname(absolutePath), { recursive: true });
  await fs.writeFile(absolutePath, file.buffer, { flag: "w" });

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
}: ListBlobParams = {}): Promise<{ data: BlobResponse[]; total: number }> {
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
    throw asHttpError("Invalid blob path", 400);
  }

  return absolutePath;
}
