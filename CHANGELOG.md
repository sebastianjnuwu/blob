# Changelog

All notable changes to this project will be documented in this file.

The format follows Keep a Changelog principles, organized by date.

## [2026-11-02]

### Fixed
- Bucket directory creation no longer validates an empty key as a file path, fixing uploads in the filesystem driver.

## [2026-10-14]

### Removed
- Legacy master token (`BLOB_TOKEN_SECRET`): auth is now only `BLOB_API_KEYS` with scopes; website token migrated to a `read,write` entry. `BLOB_SIGNED_URL_SECRET` (HMAC) unchanged.

### Added
- API keys in the database (`api_keys`: sha256 hash, scopes, revoke) + persisted HMAC secret (`settings`): `MAIL_ADMIN` labels keys created via CLI (`blob admin create`, shows the token once; `list`/`revoke`). No HTTP admin routes and no auto-generation on boot. Removes `BLOB_API_KEYS`/`BLOB_SIGNED_URL_SECRET` from env.
- S3 compatibility (auth + objects): SigV4 verification (header `Authorization` or presigned query, stdlib, tested against real signer `@smithy/signature-v4`) accepted on all private routes alongside Bearer; shim `PUT/GET/DELETE/HEAD /{bucket}/{key}`, `CreateBucket`, `ListBuckets`/`ListObjectsV2` (XML), ETag/Content-Length/Range. Working demos in `examples/s3/js` and `examples/s3/go`.
- S3-style credentials (`access_key_id` + `secret`, new columns in `api_keys`): `blob admin create --label X --scopes ... --s3` (`--bearer` is the default; imports via `--access-key/--secret` or `--token`); `list` shows the access key; secret stays on the server for SigV4 verification. Current Bearer unchanged.
- Complete S3 shim for SDKs: `CopyObject` (PUT + `X-Amz-Copy-Source`, shares file via refcount, `REPLACE` of metadata), overwrite on same-key PUT, `X-Amz-Acl: public-read` + anonymous GET/HEAD, `X-Blob-Expires-At` with enforcement, presigned URLs, S3 multipart (`Create/UploadPart/Complete/Abort`, `ListParts`, `ListMultipartUploads`), JS and Go demos validated live.

### Changed
- Logger unified with the API's (`src/lib/logger`): color tags (`<red>`, `<cyan>`...), categories (`Server`, `Database`, `Environment`, `HTTP`, `Queue`), `StatusCode` helper. Calls migrated from `functions.Info/Warn/Error/Debug` (16 files + `main.go`); `src/functions/logger.go` removed.

## [2026-06-29]

### Added
- Added `view_count` tracking for inline file views (`/blob/:id/view`).

### Changed
- Updated file serving flow to use explicit counters per route:
	- `/blob/:id/download` increments `download_count`
	- `/blob/:id/view` increments `view_count`
- Metrics summary now includes `total_views`.
- Updated API documentation examples and schema with `view_count`.

### Fixed
- Added legacy-safe counter increment using `COALESCE(..., 0) + 1`.
- Added startup backfill to normalize `NULL` values in `download_count` and `view_count` to `0`.
