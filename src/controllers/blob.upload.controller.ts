import type { NextFunction, Request, Response } from "express";
import { z } from "zod";
import { saveBlob } from "#services/blob.service";
import { hasAdminAccess } from "#controllers/blob.util";

/**
 * Uploads a new blob (file) to storage.
 *
 * @route POST /blob/upload
 * @summary Upload a file to the blob storage. Requires admin authentication via `x-admin-token` or `Authorization: Bearer` header.
 *
 * @param req Express request (multipart/form-data)
 *   - file: File (required, field name: `file`)
 *   - bucket: string (optional, logical grouping)
 *   - key: string (optional, custom identifier)
 *   - public: boolean (optional, if true, file is public)
 *   - metadata: JSON string (optional, custom metadata)
 *   - expiresAt: ISO string or timestamp (optional, expiration date)
 * @param res Express response
 * @param next Express error callback
 *
 * @returns 201 Created: Blob metadata (JSON)
 * @returns 400 Bad Request: Missing file or invalid input
 * @returns 401 Unauthorized: Missing/invalid admin token
 *
 * @example Request (multipart/form-data)
 *   POST /blob/upload
 *   Headers: x-admin-token: <TOKEN>
 *   Body:
 *     file: <file>
 *     bucket: "images"
 *     public: true
 *     metadata: '{"description":"My image"}'
 *
 * @example Response (201)
 *   {
 *     "id": "abc123",
 *     "filename": "myfile.png",
 *     "bucket": "images",
 *     "public": true,
 *     "metadata": {"description":"My image"},
 *     ...
 *   }
 *
 * @security AdminToken
 * @see hasAdminAccess
 */
export async function uploadBlob(
    req: Request,
    res: Response,
    next: NextFunction,
): Promise<void> {
    try {
        if (!hasAdminAccess(req)) {
            res
                .status(401)
                .json({ error: "Administrative token required for upload" });
            return;
        }
        if (!req.file) {
            res.status(400).json({ error: "Missing file field in multipart body" });
            return;
        }
        const schema = z.object({
            bucket: z.string().optional(),
            key: z.string().optional(),
            public: z
                .preprocess((v) => v === "true" || v === true, z.boolean())
                .optional(),
            metadata: z.string().optional(),
            expiresAt: z.string().datetime().optional(),
        });
        const parsed = schema.safeParse(req.body);
        if (!parsed.success) {
            res
                .status(400)
                .json({ error: "Invalid input", details: parsed.error.flatten() });
            return;
        }
        let expiresAt: Date | undefined;
        if (parsed.data.expiresAt) {
            const date = new Date(parsed.data.expiresAt);
            if (!Number.isNaN(date.getTime())) {
                expiresAt = date;
            }
        }
        const blob = await saveBlob(req.file, {
            bucket: parsed.data.bucket,
            key: parsed.data.key,
            isPublic: parsed.data.public,
            metadata: parsed.data.metadata,
            expiresAt,
        });
        // Monta a URL de acesso ao blob
        const baseUrl = req.protocol + '://' + req.get('host');
        const url = `${baseUrl}/blob/${blob.id}`;
        res.status(201).json({ ...blob, url });
        return;
    } catch (error) {
        next(error);
        return;
    }
}
