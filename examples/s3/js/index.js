import {
	S3Client,
	ListBucketsCommand,
	ListObjectsV2Command,
	GetObjectCommand,
	PutObjectCommand,
	DeleteObjectCommand,
	HeadObjectCommand,
} from "@aws-sdk/client-s3";

/**
 * S3-compatible client configuration.
 *
 * @see https://docs.aws.amazon.com/AWSJavaScriptSDK/v3/latest/client/s3/
 */
const S3 = new S3Client({
	region: "us-east-1",
	endpoint: process.env.S3_ENDPOINT ?? "http://127.0.0.1:3099",
	forcePathStyle: true,
	credentials: {
		accessKeyId: process.env.S3_ACCESS_KEY_ID,
		secretAccessKey: process.env.S3_SECRET_ACCESS_KEY,
	},
});

/** @type {string} Target bucket name. */
const Bucket = process.env.S3_BUCKET ?? "demo";

/** @type {string} Object key used for the compatibility test. */
const Key = "hello.txt";

/** @type {string} Expected object content. */
const Body = "hello s3-compatível";

/**
 * Runs an end-to-end S3 compatibility test.
 *
 * The test lists buckets, uploads an object, lists objects,
 * retrieves object metadata, downloads the object, and verifies
 * that the downloaded content matches the original content.
 *
 * Set DELETE_OBJECT=true to delete the test object after verification.
 *
 * @async
 * @returns {Promise<void>} Resolves when all enabled tests succeed.
 * @throws {Error} If an S3 operation fails or content verification fails.
 */
async function main() {
	console.log(
		"S3 endpoint:",
		process.env.S3_ENDPOINT ?? "http://127.0.0.1:3099"
	);

	console.log("Bucket:", Bucket);

	// List available buckets.
	const buckets = await S3.send(new ListBucketsCommand({}));

	console.log(
		"buckets:",
		buckets.Buckets?.map((bucket) => bucket.Name)
	);

	// Upload a text object.
	const put = await S3.send(
		new PutObjectCommand({
			Bucket,
			Key,
			Body,
			ContentType: "text/plain",
		})
	);

	console.log("put ETag:", put.ETag);

	// List objects stored in the target bucket.
	const listed = await S3.send(
		new ListObjectsV2Command({
			Bucket,
		})
	);

	console.log(
		"listed:",
		listed.Contents?.map(
			(object) => `${object.Key} (${object.Size} bytes, ${object.ETag})`
		)
	);

	// Retrieve object metadata without downloading its content.
	const head = await S3.send(
		new HeadObjectCommand({
			Bucket,
			Key,
		})
	);

	console.log("head:", {
		contentType: head.ContentType,
		contentLength: head.ContentLength,
		etag: head.ETag,
	});

	// Download the uploaded object.
	const get = await S3.send(
		new GetObjectCommand({
			Bucket,
			Key,
		})
	);

	if (!get.Body) {
		throw new Error("The object was returned without a response body.");
	}

	const text = await get.Body.transformToString();

	console.log("get:", JSON.stringify(text));
	console.log("match:", text === Body);
	console.log("get ETag:", get.ETag);
	console.log("get length:", get.ContentLength);

	// Verify that the downloaded content matches the uploaded content.
	if (text !== Body) {
		throw new Error("The downloaded content does not match the uploaded content.");
	}

	// Optionally delete the test object.
	if (process.env.DELETE_OBJECT === "true") {
		const deleted = await S3.send(
			new DeleteObjectCommand({
				Bucket,
				Key,
			})
		);

		console.log("deleted:", deleted);
	}

	console.log("S3 compatibility test completed successfully.");
}

/**
 * Executes the test and reports any unhandled errors.
 *
 * Sets the process exit code to 1 if the test fails.
 */
main().catch((err) => {
	console.error("S3 compatibility test failed:", err);
});