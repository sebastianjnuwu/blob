// S3 compatibility demo against the Blob caBRAPI shim.
//
// Env: S3_ENDPOINT (default http://127.0.0.1:3099), S3_ACCESS_KEY_ID,
// S3_SECRET_ACCESS_KEY, S3_BUCKET (default demo). Set DELETE_OBJECT=true
// to delete the test object after verification.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func check(err error, op string) {
	if err != nil {
		fmt.Fprintln(os.Stderr, op+" failed:", err)
		os.Exit(1)
	}
}

func main() {
	ctx := context.Background()
	endpoint := env("S3_ENDPOINT", "http://127.0.0.1:3099")
	bucket := env("S3_BUCKET", "demo")
	key := "hello.txt"
	body := "hello s3-compatível"

	client := s3.NewFromConfig(aws.Config{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider(env("S3_ACCESS_KEY_ID", ""), env("S3_SECRET_ACCESS_KEY", ""), ""),
		BaseEndpoint: aws.String(endpoint),
	}, func(o *s3.Options) { o.UsePathStyle = true })

	fmt.Println("S3 endpoint:", endpoint)

	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	check(err, "ListBuckets")
	names := []string{}
	for _, b := range out.Buckets {
		names = append(names, aws.ToString(b.Name))
	}
	fmt.Println("buckets:", names)

	put, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucket, Key: &key, Body: bytes.NewReader([]byte(body)), ContentType: aws.String("text/plain"),
	})
	check(err, "PutObject")
	fmt.Println("put ETag:", aws.ToString(put.ETag))

	listed, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &bucket})
	check(err, "ListObjectsV2")
	for _, c := range listed.Contents {
		fmt.Printf("listed: %s (%db %s)\n", aws.ToString(c.Key), aws.ToInt64(c.Size), aws.ToString(c.ETag))
	}

	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &bucket, Key: &key})
	check(err, "HeadObject")
	fmt.Println("head len:", aws.ToInt64(head.ContentLength))

	get, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	check(err, "GetObject")
	got, err := io.ReadAll(get.Body)
	check(err, "read body")
	get.Body.Close()
	fmt.Printf("get: %q | match: %t\n", string(got), string(got) == body)

	if os.Getenv("DELETE_OBJECT") == "true" {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &bucket, Key: &key})
		check(err, "DeleteObject")
		fmt.Println("deleted")
	}
}
