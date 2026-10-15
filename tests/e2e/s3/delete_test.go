package s3

import (
	"net/http"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func TestDelete(t *testing.T) {
	bucket := suite.Bucket(t, "del")
	if code, _ := signedPut(t, bucket, "a.txt", "bye", "", nil); code != 200 {
		t.Fatalf("put: %d", code)
	}

	req, _ := http.NewRequest("DELETE", suite.Base+"/"+bucket+"/a.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 204 {
		t.Fatalf("delete: %d", code)
	}

	// idempotent: second delete still 204
	req, _ = http.NewRequest("DELETE", suite.Base+"/"+bucket+"/a.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 204 {
		t.Fatalf("delete again: %d", code)
	}

	// gone afterwards
	req, _ = http.NewRequest("GET", suite.Base+"/"+bucket+"/a.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 404 {
		t.Fatalf("get after delete: %d", code)
	}
}

func TestDeleteBucket(t *testing.T) {
	bucket := suite.Bucket(t, "delbucket")

	// create + HEAD bucket
	req, _ := http.NewRequest("PUT", suite.Base+"/"+bucket+"/", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 200 {
		t.Fatalf("create bucket: %d", code)
	}
	req, _ = http.NewRequest("HEAD", suite.Base+"/"+bucket+"/", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 200 {
		t.Fatalf("head bucket: %d", code)
	}

	// non-empty delete -> 409
	if code, _ := signedPut(t, bucket, "a.txt", "x", "", nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	req, _ = http.NewRequest("DELETE", suite.Base+"/"+bucket, nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 409 {
		t.Fatalf("delete non-empty bucket: %d", code)
	}

	// empty it, then 204
	s3delete(t, bucket, "a.txt")
	req, _ = http.NewRequest("DELETE", suite.Base+"/"+bucket, nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 204 {
		t.Fatalf("delete empty bucket: %d", code)
	}
}
