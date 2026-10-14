import {
	S3Client,
	ListBucketsCommand,
	ListObjectsV2Command,
	GetObjectCommand,
	PutObjectCommand,
} from "@aws-sdk/client-s3";

// Demo S3 contra o Blob caBRAPI (shim S3 em localhost).
// Credenciais via env (criadas com `blob admin create-s3`):
//   S3_ACCESS_KEY_ID / S3_SECRET_ACCESS_KEY
const S3 = new S3Client({
	region: "us-east-1",
	endpoint: "http://127.0.0.1:3099",
	forcePathStyle: true,
	credentials: {
		accessKeyId: process.env.S3_ACCESS_KEY_ID,
		secretAccessKey: process.env.S3_SECRET_ACCESS_KEY,
	},
});

const Bucket = "demo";
const Key = "hello.txt";
const Body = "hello s3-compatível";

const buckets = await S3.send(new ListBucketsCommand({}));
console.log("buckets:", buckets.Buckets?.map((b) => b.Name));

const put = await S3.send(new PutObjectCommand({ Bucket, Key, Body, ContentType: "text/plain" }));
console.log("put ETag:", put.ETag);

const listed = await S3.send(new ListObjectsV2Command({ Bucket }));
console.log("listed:", listed.Contents?.map((c) => `${c.Key} (${c.Size}b ${c.ETag})`));

const get = await S3.send(new GetObjectCommand({ Bucket, Key }));
const text = await get.Body.transformToString();
console.log("get:", JSON.stringify(text), "| match:", text === Body);
console.log("get ETag:", get.ETag, "| len:", get.ContentLength);
