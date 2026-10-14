# Blob (Binary Large Object)

Simple HTTP service for storing and retrieving binary files (blobs) with metadata.

## Docker Image

![CI/CD](https://github.com/sebastianjnuwu/blob/actions/workflows/ci.yml/badge.svg)

This project automatically builds and publishes a Docker image to **GitHub Container Registry (GHCR)** on every push to the `main` branch.

### Pull the latest image

```bash
docker pull ghcr.io/sebastianjnuwu/blob:latest
```

### Run the container

```bash
docker run -d \
-p 3000:3000 \
--env-file .env \
-v ./storage:/storage \
--name blob \
ghcr.io/sebastianjnuwu/blob:latest
```

### What these options do

* `-d`
  Runs the container in the background.

* `-p 3000:3000`
  Maps port **3000 of the container** to **port 3000 on the host**.

* `--env-file .env`
  Loads environment variables from the `.env` file.

* `-v ./storage:/storage`
  Mounts a persistent storage directory so uploaded files are not lost.

* `--name blob`
  Gives the container an easy name for management.

## Authentication

There is no master token and no secret in env. API keys live in the database
and are managed with the CLI (shown once at creation, only hashes are stored):

```bash
# bearer key
go run . admin create --label website --scopes read,write
# S3-style credentials (access key + secret)
go run . admin create-s3 --label backup --scopes read,write
# list / revoke
go run . admin list
go run . admin revoke <id>
```

Private routes accept a bearer token or an AWS SigV4 signature:

```bash
curl -H "Authorization: Bearer <api-key>" http://localhost:3000/blob
```

The server is S3-compatible: standard AWS SDKs work against it (SigV4 +
`PUT/GET/DELETE/HEAD /{bucket}/{key}`, buckets, `ListBuckets`/`ListObjectsV2`).
See `examples/s3/` for a working demo with `@aws-sdk/client-s3`.

## Routes

| Method | Route                          | Private | Description                          |
| ------ | ------------------------------ | ------- | ------------------------------------ |
| PUT    | `/blob`                        | true    | Upload a blob                        |
| POST   | `/blob/initiate`               | true    | Initiate multipart upload (huge file) |
| PUT    | `/blob/:id/chunk`       | true    | Upload a chunk (multipart)            |
| POST   | `/blob/:id/complete`    | true    | Complete multipart upload             |
| GET    | `/blob/:id/status`      | true    | Check multipart upload status         |
| GET    | `/blob`                        | true    | List blobs                            |
| GET    | `/blob/:id`                    | true    | Get blob metadata                     |
| POST   | `/blob/:id`                    | true    | Edit blob fields                      |
| POST   | `/blob/:id/sign`               | true    | Generate signed download/view URLs    |
| GET    | `/blob/:id/download`           | false   | Download blob file (supports Range)   |
| GET    | `/blob/:id/view`               | false   | View blob inline (opens in browser)   |
| DELETE | `/blob/:id`                    | true    | Delete blob                           |
| PUT    | `/{bucket}/{key}`              | true    | S3 PutObject                          |
| GET    | `/{bucket}/{key}`              | true    | S3 GetObject (supports Range)         |
| DELETE | `/{bucket}/{key}`              | true    | S3 DeleteObject                       |
| HEAD   | `/{bucket}/{key}`              | true    | S3 HeadObject                         |
| PUT    | `/{bucket}/`                   | true    | S3 CreateBucket (implicit, no-op)     |
| GET    | `/`                            | false   | S3 ListBuckets (with auth) / Hello    |
| GET    | `/{bucket}?list-type=2`        | true    | S3 ListObjectsV2                      |

### S3 compatibility

Besides Bearer tokens, every private route accepts AWS SigV4 signatures, so
standard AWS SDKs work without custom clients:

```js
import { S3Client, PutObjectCommand } from "@aws-sdk/client-s3";

const S3 = new S3Client({
  region: "us-east-1",
  endpoint: "http://localhost:3000",
  forcePathStyle: true, // path style: /bucket/key (not bucket.host/key)
  credentials: { accessKeyId: "<access-key>", secretAccessKey: "<secret>" },
});

await S3.send(new PutObjectCommand({ Bucket: "demo", Key: "hello.txt", Body: "hi" }));
```

Create S3 credentials with `go run . admin create-s3 --label demo --scopes read,write`.
A runnable demo lives in `examples/s3/` (needs `npm ci` there first).

Notes: ETags carry the content hash (opaque to clients); buckets are implicit
(a bucket exists once it holds an object); `ListBuckets` derives names from
stored objects.
| GET    | `/metrics`                     | true    | Storage and usage metrics (JSON)      |
| GET    | `/health`                      | false   | Healthcheck (DB, Redis, storage)      |
| GET    | `/version`                     | false   | Build version                         |
| GET    | `/`                            | false   | Hello, World                          |

## Database Schema

| Column         | Type        | Nullable | Description                  |
| -------------- | ----------- | -------- | ---------------------------- |
| id             | UUID        | No       | Unique identifier            |
| bucket         | TEXT        | No       | Logical group of files       |
| filename       | TEXT        | No       | File name                    |
| mime           | TEXT        | No       | MIME type                    |
| size           | BIGINT      | No       | File size in bytes           |
| hash           | TEXT        | No       | SHA256 hash of the file      |
| path           | TEXT        | No       | Storage path                 |
| public         | BOOLEAN     | Yes      | Whether blob is public       |
| download_count | INT         | No       | Number of downloads          |
| view_count     | INT         | No       | Number of inline views       |
| metadata       | JSONB       | Yes      | Additional metadata (JSON)   |
| created_at     | TIMESTAMPTZ | No       | Creation timestamp           |
| updated_at     | TIMESTAMPTZ | No       | Last update timestamp        |
| expires_at     | TIMESTAMPTZ | Yes      | Optional expiration date     |
| deleted_at     | TIMESTAMPTZ | Yes      | Soft delete timestamp        |

## API keys (`api_keys` table)

| Column     | Type        | Nullable | Description                        |
| ---------- | ----------- | -------- | ---------------------------------- |
| id         | UUID        | No       | Unique identifier                  |
| label      | TEXT        | No       | Human label (e.g. an admin email)  |
| token_hash | TEXT        | Yes      | SHA256 of bearer tokens (one side) |
| access_key_id | TEXT     | Yes      | S3 access key id (other side)  |
| secret     | TEXT        | Yes      | S3 secret, server-side only        |
| scopes     | TEXT        | No       | `read,write,delete,all,admin`      |
| active     | BOOLEAN     | No       | Revoke by setting false            |
| created_at | TIMESTAMPTZ | No       | Creation timestamp                 |
| updated_at | TIMESTAMPTZ | No       | Last update timestamp              |

Server-side secrets (e.g. the HMAC key for signed URLs) live in the
`settings` table (`key`/`value`) and are created automatically on first boot.

## Usage Examples

### GET `/`

```bash
curl http://localhost:3000/
```

Response:

```json
{
  "message": "Hello, World!"
}
```

### GET `/health`

```bash
curl http://localhost:3000/health
```

Response:

```json
{
  "status": "ok"
}
```

### GET `/metrics`

#### Example

```bash
curl -X GET http://localhost:3000/metrics \
  -H "Authorization: Bearer <api-key>"
```

Response:
```json
{
    "buckets": [
        {
            "blobs": 9,
            "name": "testbucket",
            "size": "8.32 MB",
            "visibility": {
                "private": 9,
                "public": 0
            }
        }
    ],
    "last_upload": {
        "bucket": "testbucket",
        "created_at": "2026-03-08T09:59:47.420216-03:00",
        "filename": "video.mp4",
        "id": "33fc6434-ab9c-43ad-b7f6-63bb7f92704c"
    },
    "summary": {
        "average_size": "946.49 KB",
        "max_size": "946.49 KB",
        "min_size": "946.49 KB",
        "multipart_completed": 4,
        "storage_free": "1015.68 MB",
        "storage_max": "1.00 GB",
        "total_blobs": 9,
        "total_downloads": 0,
        "total_views": 0,
        "total_size": "8.32 MB"
    },
    "types": [
        {
            "count": 9,
            "mime": "video/mp4",
            "size": "8.32 MB"
        }
    ]
}
```

### PUT `/blob`

Uploads a new blob.

#### Accepted Fields

| Field      | Required | Type    | Description                                                        |
| ---------- | -------- | ------- | ------------------------------------------------------------------ |
| file       | Yes      | file    | File to upload                                                     |
| bucket     | Yes      | string  | Logical group/path (supports subfolders, e.g., "folder/subfolder") |
| filename   | No       | string  | Custom filename                                                    |
| public     | No       | boolean | Accepts true, false, 0, 1 (default: true)                          |
| expires_at | No       | string  | RFC3339 date                                                       |
| metadata   | No       | string  | JSON metadata                                                      |

#### Example

```bash
curl -X PUT http://localhost:3000/blob \
  -H "Authorization: Bearer <api-key>" \
  -F "file=@README.md" \
  -F "bucket=folder/subfolder" \
  -F "filename=custom_name.txt" \
  -F "public=false" \
  -F "expires_at=2026-03-02T12:00:00Z" \
  -F "metadata={\"author\":\"user\",\"desc\":\"test file\"}"
```

Response:

```json
{
  "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "bucket": "test",
  "filename": "custom_name.txt",
  "mime": "application/octet-stream",
  "size": 3625,
  "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "path": "test/1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "public": false,
  "download_count": 0,
  "view_count": 0,
  "created_at": "2026-03-07T12:31:05.2082654-03:00",
  "updated_at": "2026-03-07T12:31:05.2082654-03:00",
  "expires_at": "2026-03-02T12:00:00Z",
  "metadata": {
    "author": "user",
    "desc": "test file"
  }
}
```


### Multipart/Chunked Upload (for huge files)

#### 1. Initiate upload:
```bash
curl -X POST http://localhost:3000/blob/initiate \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{"bucket":"bigfiles","filename":"video_20tb.mkv","size":21990232555520}'
# Response: { "uploadId": "abc123" }
```

Session ownership is derived from the authenticated `Authorization` bearer
token, so there is no client-supplied user identifier. The declared `size`
must not exceed `BLOB_MAX_UPLOAD_SIZE_BYTES` and is checked against
`BLOB_MAX_STORAGE_SIZE` at initiate time.

#### 2. Upload each chunk (with integrity check):
```bash
CHUNK_HASH=$(sha256sum chunk_0.bin | awk '{print $1}')
curl -X PUT http://localhost:3000/blob/abc123/chunk \
  -H "Authorization: Bearer <api-key>" \
  -H "X-Chunk-Index: 0" \
  -H "X-Chunk-Hash: $CHUNK_HASH" \
  --data-binary "@chunk_0.bin"
# Repeat for each chunk, incrementing index and hash
```

Chunks accumulate against `BLOB_MAX_UPLOAD_SIZE_BYTES`; exceeding it rejects
the chunk.

#### 3. (Optional) Check status:
```bash
curl -H "Authorization: Bearer <api-key>" http://localhost:3000/blob/abc123/status
```

#### 4. Complete upload (with final hash):
```bash
FINAL_HASH=$(sha256sum full_file.bin | awk '{print $1}')
curl -X POST http://localhost:3000/blob/abc123/complete \
  -H "Authorization: Bearer <api-key>" \
  -H "X-Final-Hash: $FINAL_HASH"
```

After completion, the file is available as a normal blob for download and management.

#### Chunk Size Limits

Configure minimum and maximum chunk size in `.env`:

```
BLOB_MIN_CHUNK_SIZE=1048576   # 1MB
BLOB_MAX_CHUNK_SIZE=20971520  # 20MB
```
Chunks outside these limits will be rejected.

#### Integrity Headers

- `X-Chunk-Hash`: Required for each chunk (SHA256 hex of chunk)
- `X-Final-Hash`: Required when completing upload (SHA256 hex of full file)

#### Automatic Cleanup

Abandoned multipart uploads are cleaned up automatically after a configurable threshold (see `.env`).

### GET `/blob`

Returns paginated blobs.

#### Query Parameters

| Parameter | Required | Type   | Description              |
| --------- | -------- | ------ | ------------------------ |
| bucket    | No       | string | Filter by bucket         |
| search    | No       | string | Search filename          |
| page      | No       | int    | Page number (default: 1) |
| page_size | No       | int    | Items per page           |

#### Example

```bash
curl "http://localhost:3000/blob?bucket=test&search=report&page=1&page_size=10"
```

Response:

```json
{
  "meta": {
    "page": 1,
    "per_page": 10,
    "count": 1,
    "pages": 1,
    "total": 42
  },
  "blobs": [
    {
      "id": "...",
      "bucket": "test",
      "filename": "report1.pdf",
      "mime": "application/pdf",
      "size": 12345,
      "hash": "...",
      "path": "test/...",
      "public": true,
      "download_count": 0,
      "view_count": 0,
      "created_at": "2026-03-07T12:31:05.2082654-03:00",
      "updated_at": "2026-03-07T12:31:05.2082654-03:00",
      "expires_at": null,
      "metadata": {
        "author": "user"
      }
    }
    // ...more blobs
  ]
}
```

### GET `/blob/:id`

Returns blob metadata as JSON (does not download the file).

#### Example

```bash
curl http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630
```

Response:

```json
{
  "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "bucket": "test",
  "filename": "custom_name.txt",
  "mime": "application/octet-stream",
  "size": 3625,
  "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "path": "test/1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "public": false,
  "download_count": 0,
  "view_count": 0,
  "created_at": "2026-03-07T12:31:05.2082654-03:00",
  "updated_at": "2026-03-07T12:31:05.2082654-03:00",
  "expires_at": "2026-03-02T12:00:00Z",
  "metadata": {
    "author": "user",
    "desc": "test file"
  }
}
```

### POST `/blob/:id`

Edits blob fields: metadata, public/private, expiration date, bucket, and filename. Requires authentication.

#### Request Body

| Field      | Required | Type    | Description                                 |
|------------|----------|---------|---------------------------------------------|
| metadata   | No       | object  | New metadata (JSON object)                  |
| public     | No       | boolean | Set blob as public or private               |
| expires_at | No       | string  | RFC3339 expiration date                     |
| bucket     | No       | string  | Change bucket name                          |
| filename   | No       | string  | Change filename                             |

#### Example

```bash
curl -X POST http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630 \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {"author": "newuser", "desc": "updated file"},
    "public": true,
    "expires_at": "2026-04-01T12:00:00Z",
    "bucket": "newbucket",
    "filename": "new_name.txt"
  }'
```

Response:

```json
{
  "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "bucket": "newbucket",
  "filename": "new_name.txt",
  "mime": "application/octet-stream",
  "size": 3625,
  "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "path": "newbucket/1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "public": true,
  "download_count": 0,
  "view_count": 0,
  "created_at": "2026-03-07T12:31:05.2082654-03:00",
  "updated_at": "2026-03-07T12:31:05.2082654-03:00",
  "expires_at": "2026-04-01T12:00:00Z",
  "metadata": {
    "author": "newuser",
    "desc": "updated file"
  }
}
```


### GET `/blob/:id/download`

#### Downloading blobs

Downloads support HTTP Range requests (`Range: bytes=start-end`), returning
`206 Partial Content`, which enables pausable downloads, video/audio seeking and
streaming of large files.

For **public blobs**, simply access the route:

```bash
curl -X GET \
  http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download \
  -o downloaded_file.ext
```

For **private blobs**, you must provide one of the following:

- A **signed URL** (recommended). Generate one with `POST /blob/:id/sign`:

  ```bash
  curl -X POST http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/sign \
    -H "Authorization: Bearer YOUR_TOKEN_HERE"
  ```

  Response:
  ```json
  {
    "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
    "expires_at": "2026-08-06T12:30:00Z",
    "ttl": 900,
    "download": "http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download?expires=1775565000&signature=<hmac>",
    "view": "http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/view?expires=1775565000&signature=<hmac>"
  }
  ```

  Then use the returned URL (it expires after `BLOB_SIGNED_URL_TTL`):

  ```bash
  curl -X GET "http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download?expires=1775565000&signature=<hmac>" \
    -o downloaded_file.ext
  ```

- The SHA256 hash of the file as a query parameter (legacy fallback):

  ```bash
  curl -X GET \
    "http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download?hash=YOUR_FILE_HASH" \
    -o downloaded_file.ext
  ```

- Or a valid authentication token in the Authorization header:

  ```bash
  curl -X GET \
    http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download \
    -H "Authorization: Bearer YOUR_TOKEN_HERE" \
    -o downloaded_file.ext
  ```

If none of these is provided for a private blob, the download will be denied.

### DELETE `/blob/:id`

Deletes a blob, its metadata, and the file from disk. Requires authentication.

#### Example

```bash
curl -X DELETE http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630 \
  -H "Authorization: Bearer <api-key>"
```

Response:

```json
{
  "message": "Blob deleted successfully"
}
```