# Blob (Binary Large OBject)

## Routes

| Method  |           Route               | Private | Description                           |
|---------|-------------------------------|---------|---------------------------------------|
| `PUT`   | `/blob`                       | `true`  | Upload Blob (full)                    |
| `POST`  | `/blob/:id/stream`            | `true`  | Upload Blob in streaming mode         |
| `GET`   | `/blob`                       | `true`  | List blobs                            |
| `GET`   | `/blob/:id`                   | `false` | Download Blob (full)                  |
| `GET`   | `/blob/:id/stream`            | `false` | Download Blob in streaming mode       |
| `HEAD`  | `/blob/:id`                   | `false` | Blob metadata                          |
| `POST`  | `/blob/:id/metadata`          | `true`  | Edit metadata                          |
| `DELETE`| `/blob/:id`                   | `true`  | Delete blob                            |
| `GET`   | `/health`                     | `false` | Healthcheck                            |
| `GET`   | `/`                           | `false` | Hello, World                            |

## Database Schema

| Column           | Type        | Nullable | Description                                      |
|------------------|------------ |--------- |-------------------------------------------------|
| `id`             | UUID        | No       | Unique identifier for each blob                 |
| `bucket`         | TEXT        | No       | Logical grouping for files                      |
| `filename`       | TEXT        | No       | Original file name                              |
| `mime`           | TEXT        | No       | File MIME type                                  |
| `size`           | BIGINT      | No       | File size in bytes                               |
| `hash`           | TEXT        | No       | Unique hash for integrity/deduplication         |
| `path`           | TEXT        | No       | Storage path or reference                       |
| `public`         | BOOLEAN     | Yes      | Whether blob is publicly accessible             |
| `download_count` | INT         | Yes      | Number of times the blob has been downloaded   |
| `metadata`       | JSONB       | Yes      | Optional metadata in JSON format (custom data) |
| `created_at`     | TIMESTAMPTZ | No       | Timestamp when blob was created                 |
| `updated_at`     | TIMESTAMPTZ | No       | Timestamp when blob was last updated            |
| `expires_at`     | TIMESTAMPTZ | Yes      | Optional expiration date for automatic cleanup |
| `deleted_at`     | TIMESTAMPTZ | Yes      | Timestamp when blob was deleted (soft delete)  |

## Usage Examples (cURL)

### GET `/`

```bash
curl -X GET "http://localhost:3000/"
```

#### response

```json
{ "message": "Hello, World!" }
```

### GET `/health`

```bash
curl -X GET "http://localhost:3000/health"
```

#### response

```json
{ "status": "ok" }
```


### PUT `/blob` (Upload)


**Accepted upload fields:**

| Field       | Required | Type     | Description                                                                 |
|-------------|----------|----------|-----------------------------------------------------------------------------|
| file        | Yes      | file     | The file to upload                                                          |
| filename    | No       | string   | Name to save the file as (default: original upload name)                     |
| bucket      | Yes      | string   | Bucket name (logical group)                                                  |
| public      | No       | boolean  | Whether the blob is public (default: true). Accepts "true", "false", "0", "1" |
| expires_at  | No       | string   | Expiration date/time in RFC3339 format (e.g. 2026-03-08T12:00:00Z)           |
| metadata    | No       | string   | JSON string with additional metadata                                         |


**Example usage:**

```bash
curl -X PUT "http://localhost:3000/blob" \
  -H "Authorization: Bearer change-me-with-32-characters-or-more" \
  -F "file=@README.md" \
  -F "bucket=test" \
  -F "filename=custom_name.txt" \
  -F "public=false" \
  -F "expires_at=2026-03-02T12:00:00Z" \
  -F "metadata={\"author\":\"user\",\"desc\":\"test file\"}"
```

#### response

```json
{
  "id": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "bucket": "test",
  "filename": "custom_name.txt",
  "mime": "application/octet-stream",
  "size": 3625,
  "hash": "1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "path": "storage/uploads/1ddff9d2-3aa1-485d-8082-e484c62ff630",
  "public": false,
  "created_at": "2026-03-07T12:31:05.2082654-03:00",
  "updated_at": "2026-03-07T12:31:05.2082654-03:00",
  "expires_at": "2026-03-02T12:00:00Z",
  "metadata": {
    "author": "user",
    "desc": "test file"
  },
  "url": "http://localhost:3000/blob/1ddff9d2-3aa1-485d-8082-e484c62ff630"
}
```
