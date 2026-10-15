import {
	S3Client,
	PutObjectCommand,
	GetObjectCommand,
	CopyObjectCommand,
	CreateMultipartUploadCommand,
	UploadPartCommand,
	CompleteMultipartUploadCommand,
	AbortMultipartUploadCommand,
} from "@aws-sdk/client-s3";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";

const S3 = new S3Client({
	region: "us-east-1",
	endpoint: "http://127.0.0.1:3099",
	forcePathStyle: true,
	credentials: {
		accessKeyId: process.env.S3_ACCESS_KEY_ID,
		secretAccessKey: process.env.S3_SECRET_ACCESS_KEY,
	},
});
const Bucket = "full";
const ok = (name, cond) => { console.log(cond ? "PASS" : "FAIL", "-", name); if (!cond) process.exitCode = 1; };

// 1. base object with public-read ACL + future expiry
const exp = new Date(Date.now() + 3600e3).toISOString();
await S3.send(new PutObjectCommand({ Bucket, Key: "pub.txt", Body: "public-data", ContentType: "text/plain", ACL: "public-read" }));
console.log("put pub.txt ok");

// 2. anonymous GET on public object (no credentials)
const Anon = new S3Client({ region: "us-east-1", endpoint: "http://127.0.0.1:3099", forcePathStyle: true, credentials: { accessKeyId: "x", secretAccessKey: "y".repeat(32) } });
try {
	const g = await Anon.send(new GetObjectCommand({ Bucket, Key: "pub.txt" }));
	ok("anon GET public", (await g.Body.transformToString()) === "public-data");
} catch (e) { ok("anon GET public", false); console.log("  err:", e.message); }

// 3. copy with REPLACE metadata
await S3.send(new PutObjectCommand({ Bucket, Key: "src.txt", Body: "v1" }));
await S3.send(new CopyObjectCommand({ Bucket, Key: "dst.txt", CopySource: `${Bucket}/src.txt`, MetadataDirective: "REPLACE", Metadata: { k: "v" } }));
const dst = await S3.send(new GetObjectCommand({ Bucket, Key: "dst.txt" }));
ok("copy REPLACE body", (await dst.Body.transformToString()) === "v1");
ok("copy REPLACE metadata", dst.Metadata?.k === "v");

// 4. presigned GET (no auth header, signed query)
const url = await getSignedUrl(S3, new GetObjectCommand({ Bucket, Key: "dst.txt" }), { expiresIn: 300 });
const pr = await fetch(url);
ok("presigned GET", pr.status === 200 && (await pr.text()) === "v1");

// 5. multipart: create + 2 parts + complete (parts >= server min 1MB)
const big = Buffer.alloc(4 * 1024 * 1024, "a");
const mp = await S3.send(new CreateMultipartUploadCommand({ Bucket, Key: "big.bin" }));
const upId = mp.UploadId;
console.log("multipart uploadId:", upId);
const p1 = await S3.send(new UploadPartCommand({ Bucket, Key: "big.bin", UploadId: upId, PartNumber: 1, Body: big.subarray(0, 2 * 1024 * 1024) }));
const p2 = await S3.send(new UploadPartCommand({ Bucket, Key: "big.bin", UploadId: upId, PartNumber: 2, Body: big.subarray(2 * 1024 * 1024) }));
const done = await S3.send(new CompleteMultipartUploadCommand({ Bucket, Key: "big.bin", UploadId: upId, MultipartUpload: { Parts: [{ ETag: p1.ETag, PartNumber: 1 }, { ETag: p2.ETag, PartNumber: 2 }] } }));
ok("multipart complete", !!done.ETag);
const gb = await S3.send(new GetObjectCommand({ Bucket, Key: "big.bin" }));
ok("multipart bytes", Number(gb.ContentLength) === big.length);

// 6. abort path
const mp2 = await S3.send(new CreateMultipartUploadCommand({ Bucket, Key: "junk.bin" }));
await S3.send(new AbortMultipartUploadCommand({ Bucket, Key: "junk.bin", UploadId: mp2.UploadId }));
console.log("abort ok");

console.log("done");
