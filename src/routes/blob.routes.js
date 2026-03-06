import express from "express";
import multer from "multer";
import {
    destroyBlob,
    getBlob,
    getBlobSignedUrl,
    listBlobs,
    uploadBlob,
} from "#controllers/blob.controller";

const router = express.Router();

const maxUploadSize = Number(
    process.env.MAX_UPLOAD_SIZE_BYTES ?? 20 * 1024 * 1024,
);

const uploadMiddleware = multer({
    storage: multer.memoryStorage(),
    limits: {
        fileSize: maxUploadSize,
        files: 1,
    },
});

router.post("/upload", uploadMiddleware.single("file"), uploadBlob);
router.get("/", listBlobs);
router.get("/:id/sign", getBlobSignedUrl);
router.get("/:id", getBlob);
router.delete("/:id", destroyBlob);

export default router;
