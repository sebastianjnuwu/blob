# Blob (Binary Large Object)

Simple HTTP service for storing and retrieving binary files (blobs) with metadata.

## Docker Image

### ![CI/CD](https://github.com/sebastianjnuwu/blob/actions/workflows/ci.yml/badge.svg)

This project automatically builds and publishes a Docker image to GitHub Container Registry (GHCR) on every push to `main`.

**To pull and run the latest image:**

```bash
docker pull ghcr.io/sebastianjnuwu/blob:latest
docker run -p 3000:3000 --env-file .env ghcr.io/sebastianjnuwu/blob:latest
```

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
| GET    | `/blob/:id/download`           | false   | Download blob file                    |
| DELETE | `/blob/:id`                    | true    | Delete blob                           |
| GET    | `/health`                      | false   | Healthcheck                           |
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
| metadata       | JSONB       | Yes      | Additional metadata (JSON)   |
| created_at     | TIMESTAMPTZ | No       | Creation timestamp           |
| updated_at     | TIMESTAMPTZ | No       | Last update timestamp        |
| expires_at     | TIMESTAMPTZ | Yes      | Optional expiration date     |
| deleted_at     | TIMESTAMPTZ | Yes      | Soft delete timestamp        |

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

### PUT `/blob`

Uploads a new blob.

#### Accepted Fields

| Field      | Required | Type    | Description                                                        |
| ---------- | -------- | ------- | ------------------------------------------------------------------ |
| file       | Yes      | file    | File to upload                                                     |
| bucket     | Yes      | string  | Logical group                                                      |
| filename   | No       | string  | Custom filename                                                    |
| public     | No       | boolean | Accepts true, false, 0, 1 (default: true)                          |
| expires_at | No       | string  | RFC3339 date                                                       |
| metadata   | No       | string  | JSON metadata                                                      |

#### Example

```bash
curl -X PUT http://localhost:3000/blob \
  -H "Authorization: Bearer change-me-with-32-characters-or-more" \
  -F "file=@README.md" \
  -F "bucket=test" \
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
  -H "Content-Type: application/json" \
  -d '{"bucket":"bigfiles","filename":"video_20tb.mkv","size":21990232555520}'
# Response: { "uploadId": "abc123" }
```

#### 2. Upload each chunk:
```bash
curl -X PUT http://localhost:3000/blob/abc123/chunk \
  -H "Content-Range: bytes 0-1073741823/21990232555520" \
  --data-binary "@chunk_0.bin"
# Repeat for each chunk, adjusting Content-Range and file
```

#### 3. (Optional) Check status:
```bash
curl http://localhost:3000/blob/abc123/status
```

#### 4. Complete upload:
```bash
curl -X POST http://localhost:3000/blob/abc123/complete
```

After completion, the file is available as a normal blob for download and management.

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
  -H "Authorization: Bearer change-me-with-32-characters-or-more" \
  -H "Content-Type: application/json" \
  -d '{
    "metadata": {"author": "newuser", "desc": "updated file"},
    "public": true,
    "expires_at": "2026-04-01T12:00:00Z",
    "filename": "new_name.txt"
  }'
```

Response:

```json
{
  "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "bucket": "bucket",
  "filename": "new_name.txt",
  "mime": "application/octet-stream",
  "size": 3625,
  "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "path": "newbucket/1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "public": true,
  "download_count": 0,
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

For **public blobs**, simply access the route:

```bash
curl -X GET \
  http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630/download \
  -o downloaded_file.ext
```

For **private blobs**, you must provide either:

- The SHA256 hash of the file as a query parameter:

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

If neither a valid hash nor a valid token is provided for a private blob, the download will be denied.

### DELETE `/blob/:id`

Deletes a blob, its metadata, and the file from disk. Requires authentication.

#### Example

```bash
curl -X DELETE http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630 \
  -H "Authorization: Bearer change-me-with-32-characters-or-more"
```

Response:

```json
{
  "message": "Blob deleted successfully"
}
```
