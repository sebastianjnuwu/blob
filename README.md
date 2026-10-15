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

There is no master token or secret stored in environment variables. API keys are stored in the database and managed through the CLI. Keys are shown only once at creation; only their hashes are stored.

```bash
go run . admin create --label test --scopes read,write [--s3 | --bearer]
go run . admin list
go run . admin revoke <id>
```

Private routes accept either a bearer token or an AWS SigV4 signature:

```bash
curl -H "Authorization: Bearer <api-key>" http://localhost:3000/blob
```

The server is S3-compatible, supporting standard AWS SDKs with SigV4 authentication
(header or presigned query string) and the operations below on `/{bucket}/{key}`,
as well as `ListBuckets` and `ListObjectsV2`.

See [`examples/s3/`](examples/s3/) for a working demo using `@aws-sdk/client-s3`.

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
| PUT    | `/{bucket}/{key}`              | true    | S3 PutObject (`X-Amz-Copy-Source` = Copy, `X-Amz-Acl: public-read`, `X-Blob-Expires-At` RFC3339) |
| GET    | `/{bucket}/{key}`              | true*   | S3 GetObject (supports Range; *anonymous when public) |
| DELETE | `/{bucket}/{key}`              | true    | S3 DeleteObject                       |
| HEAD   | `/{bucket}/{key}`              | true*   | S3 HeadObject (*anonymous when public) |
| PUT    | `/{bucket}/`                   | true    | S3 CreateBucket (implicit, no-op)     |
| GET    | `/`                            | false   | S3 ListBuckets (with auth) / Hello    |
| GET    | `/{bucket}?list-type=2`        | true    | S3 ListObjectsV2                      |
| POST   | `/{bucket}/{key}?uploads`      | true    | S3 CreateMultipartUpload              |
| PUT    | `/{bucket}/{key}?partNumber=N&uploadId=U` | true | S3 UploadPart (server-hashed, min `BLOB_MIN_CHUNK_SIZE`) |
| POST   | `/{bucket}/{key}?uploadId=U`   | true    | S3 CompleteMultipartUpload            |
| DELETE | `/{bucket}/{key}?uploadId=U`   | true    | S3 AbortMultipartUpload               |
| GET    | `/{bucket}/{key}?uploadId=U`   | true    | S3 ListParts (no ETags: no cross-process resume) |
| GET    | `/{bucket}?uploads`            | true    | S3 ListMultipartUploads               |

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

### S3-Compatible API

Blob supports S3-compatible clients through AWS Signature Version 4 (SigV4), allowing applications to interact with stored objects using standard AWS SDKs.

#### Configuration

Set the following environment variables:

```env
S3_ENDPOINT=http://localhost:3000
S3_ACCESS_KEY_ID=your-access-key
S3_SECRET_ACCESS_KEY=your-secret-key
S3_BUCKET=demo
```

Create S3 credentials using the CLI:

```bash
go run . admin create --label demo --scopes read,write --s3
```

Install the AWS SDK:

```bash
npm install @aws-sdk/client-s3 @aws-sdk/s3-request-presigner
```

The examples below use Node.js with ES modules.

#### Initialize the Client

Create `examples/s3/s3.js`:

```javascript
import { S3Client } from "@aws-sdk/client-s3";

export const s3 = new S3Client({
  region: "us-east-1",
  endpoint: process.env.S3_ENDPOINT,
  forcePathStyle: true,
  credentials: {
    accessKeyId: process.env.S3_ACCESS_KEY_ID,
    secretAccessKey: process.env.S3_SECRET_ACCESS_KEY,
  },
});
```

Make sure the environment variables are loaded before running the examples.

#### List Buckets

Retrieve the buckets available through the S3 API:

```javascript
import { ListBucketsCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const response = await s3.send(new ListBucketsCommand({}));

console.log(response.Buckets?.map((bucket) => bucket.Name));
```

#### Upload a Text Object

Upload a small text object without creating a local file:

```javascript
import { PutObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const response = await s3.send(
  new PutObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "hello.txt",
    Body: "Hello from Blob!",
    ContentType: "text/plain; charset=utf-8",
  })
);

console.log("Object uploaded successfully.");
console.log("ETag:", response.ETag);
```

#### Upload a File

Upload a local file using a readable stream:

```javascript
import { createReadStream, statSync } from "node:fs";
import { basename } from "node:path";
import { PutObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const filePath = "./files/document.pdf";
const fileName = basename(filePath);

const response = await s3.send(
  new PutObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: fileName,
    Body: createReadStream(filePath),
    ContentLength: statSync(filePath).size,
    ContentType: "application/pdf",
  })
);

console.log("File uploaded successfully.");
console.log("Key:", fileName);
console.log("ETag:", response.ETag);
```

The object key can include directories, such as `documents/2026/document.pdf`. These are key prefixes, not necessarily real filesystem directories or independently created folders.

#### Download an Object

Download an object and save it to disk:

```javascript
import { createWriteStream } from "node:fs";
import { mkdir } from "node:fs/promises";
import { dirname } from "node:path";
import { pipeline } from "node:stream/promises";
import { GetObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const key = "document.pdf";
const destination = "./downloads/document.pdf";

const response = await s3.send(
  new GetObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: key,
  })
);

if (!response.Body) {
  throw new Error("The object has no response body.");
}

await mkdir(dirname(destination), { recursive: true });
await pipeline(response.Body, createWriteStream(destination));

console.log("File downloaded successfully.");
console.log("Content-Type:", response.ContentType);
console.log("Content-Length:", response.ContentLength);
```

#### Read an Object as Text

Retrieve a text object directly into memory:

```javascript
import { GetObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const response = await s3.send(
  new GetObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "hello.txt",
  })
);

if (!response.Body) {
  throw new Error("The object has no response body.");
}

const content = await response.Body.transformToString();

console.log(content);
```

Use this approach for small text objects. For large files, prefer streaming the response to disk.

#### List Objects

List objects in a bucket:

```javascript
import { ListObjectsV2Command } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const response = await s3.send(
  new ListObjectsV2Command({
    Bucket: process.env.S3_BUCKET,
    Prefix: "",
    MaxKeys: 100,
  })
);

console.log(
  response.Contents?.map((object) => ({
    key: object.Key,
    size: object.Size,
    lastModified: object.LastModified,
    etag: object.ETag,
  }))
);

console.log("More objects:", response.IsTruncated);
```

Use `Prefix` to filter by key prefix:

```javascript
const response = await s3.send(
  new ListObjectsV2Command({
    Bucket: process.env.S3_BUCKET,
    Prefix: "documents/",
    MaxKeys: 100,
  })
);
```

#### List All Objects with Pagination

For buckets containing many objects, use the continuation token to retrieve subsequent pages:

```javascript
import { ListObjectsV2Command } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

let continuationToken;

do {
  const response = await s3.send(
    new ListObjectsV2Command({
      Bucket: process.env.S3_BUCKET,
      MaxKeys: 1000,
      ContinuationToken: continuationToken,
    })
  );

  for (const object of response.Contents ?? []) {
    console.log({
      key: object.Key,
      size: object.Size,
      lastModified: object.LastModified,
    });
  }

  continuationToken = response.IsTruncated
    ? response.NextContinuationToken
    : undefined;
} while (continuationToken);
```

This example assumes the server implements standard `ListObjectsV2` continuation-token behavior.

#### Check Whether an Object Exists

Use `HeadObject` to check object availability without downloading its contents:

```javascript
import { HeadObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

try {
  const response = await s3.send(
    new HeadObjectCommand({
      Bucket: process.env.S3_BUCKET,
      Key: "document.pdf",
    })
  );

  console.log("Object exists.");
  console.log("Size:", response.ContentLength);
  console.log("Content-Type:", response.ContentType);
} catch (error) {
  if (error.$metadata?.httpStatusCode === 404) {
    console.log("Object not found.");
  } else {
    throw error;
  }
}
```

#### Get Object Metadata

Retrieve metadata without downloading the object:

```javascript
import { HeadObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

const response = await s3.send(
  new HeadObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "document.pdf",
  })
);

console.log({
  contentType: response.ContentType,
  contentLength: response.ContentLength,
  lastModified: response.LastModified,
  etag: response.ETag,
  metadata: response.Metadata,
});
```

#### Copy an Object

Copy an object to another key within the same bucket:

```javascript
import { CopyObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

await s3.send(
  new CopyObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "documents/document-copy.pdf",
    CopySource: `${process.env.S3_BUCKET}/document.pdf`,
  })
);

console.log("Object copied successfully.");
```

This operation requires server-side `CopyObject` support. URL-encode the source key when it contains special characters.

#### Generate a Presigned Download URL

Generate a temporary URL that allows a client to download an object without receiving your S3 credentials:

```javascript
import { GetObjectCommand } from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { s3 } from "./s3.js";

const url = await getSignedUrl(
  s3,
  new GetObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "document.pdf",
  }),
  { expiresIn: 900 }
);

console.log(url);
```

The URL is intended to remain valid for 15 minutes. This requires the server to support the corresponding presigned SigV4 requests.

#### Generate a Presigned Upload URL

Allow a client to upload a file directly without exposing your credentials:

```javascript
import { PutObjectCommand } from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { s3 } from "./s3.js";

const url = await getSignedUrl(
  s3,
  new PutObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "uploads/image.png",
    ContentType: "image/png",
  }),
  { expiresIn: 900 }
);

console.log(url);
```

The client must send a `PUT` request to the generated URL. If `ContentType` is included when signing, the upload must use the corresponding content type. This requires support for presigned SigV4 requests.

#### Multipart Upload

Upload a large file in parts (each part must respect `BLOB_MIN_CHUNK_SIZE`):

```javascript
import {
  CreateMultipartUploadCommand,
  UploadPartCommand,
  CompleteMultipartUploadCommand,
} from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";
import { readFileSync } from "node:fs";

const Bucket = process.env.S3_BUCKET;
const Key = "video.mp4";
const Body = readFileSync("./video.mp4");
const half = Math.ceil(Body.length / 2);

const { UploadId } = await s3.send(new CreateMultipartUploadCommand({ Bucket, Key }));
const parts = [];
for (const [i, chunk] of [Body.subarray(0, half), Body.subarray(half)].entries()) {
  const { ETag } = await s3.send(
    new UploadPartCommand({ Bucket, Key, UploadId, PartNumber: i + 1, Body: chunk })
  );
  parts.push({ ETag, PartNumber: i + 1 });
}
await s3.send(new CompleteMultipartUploadCommand({ Bucket, Key, UploadId, MultipartUpload: { Parts: parts } }));

console.log("Multipart upload completed.");
```

Abort with `AbortMultipartUploadCommand`; list parts/uploads with `ListPartsCommand` / `ListMultipartUploadsCommand`.

#### Delete an Object

Delete an object by its key:

```javascript
import { DeleteObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

await s3.send(
  new DeleteObjectCommand({
    Bucket: process.env.S3_BUCKET,
    Key: "document.pdf",
  })
);

console.log("Delete request completed.");
```

S3-compatible delete operations may succeed even when the object does not exist. Check the server's behavior if you need to distinguish those cases.

#### Error Handling

Handle common errors when interacting with Blob:

```javascript
import { GetObjectCommand } from "@aws-sdk/client-s3";
import { s3 } from "./s3.js";

try {
  await s3.send(
    new GetObjectCommand({
      Bucket: process.env.S3_BUCKET,
      Key: "missing.txt",
    })
  );
} catch (error) {
  console.error("S3 request failed:", error.message);
  console.error("HTTP status:", error.$metadata?.httpStatusCode);
  console.error("Request ID:", error.$metadata?.requestId);
}
```

Use the HTTP status code and error details to distinguish missing objects, invalid credentials, insufficient permissions, and other failures.

### Compatibility Notes

- Private S3 operations use AWS Signature Version 4 (SigV4), via header or presigned query string.
- Use `forcePathStyle: true` when connecting to a custom endpoint.
- Buckets are implicit (a bucket exists once it holds an object); `CreateBucket` returns 200 as a no-op.
- ETags carry the content hash (opaque to clients, not MD5).
- Multipart uploads work through S3 (`CreateMultipartUpload`/`UploadPart`/`Complete`/`Abort`, `ListParts`/`ListMultipartUploads`); parts must respect `BLOB_MIN_CHUNK_SIZE`, and `ListParts` omits ETags (no cross-process resume).
- Copy with `X-Amz-Metadata-Directive: REPLACE` swaps metadata; default copies it.
- `X-Amz-Acl: public-read` makes objects world-readable (anonymous GET/HEAD); `X-Blob-Expires-At` (RFC3339) sets per-object expiry, enforced as `NoSuchKey`.
- Keep access keys and secret keys on the server. Do not expose them in frontend applications.

For additional service-specific endpoints, including health checks, metrics, and Blob's chunked upload workflow, see the corresponding API documentation.