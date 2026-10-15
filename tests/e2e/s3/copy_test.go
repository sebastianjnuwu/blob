package s3

import (
	"net/http"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func TestCopy(t *testing.T) {
	bucket := suite.Bucket(t, "copy")
	if code, _ := signedPut(t, bucket, "src.txt", "v1", "text/plain", map[string]string{"X-Amz-Meta-K": "orig"}); code != 200 {
		t.Fatalf("put: %d", code)
	}
	defer s3delete(t, bucket, "src.txt")
	defer s3delete(t, bucket, "dst.txt")

	// default COPY keeps metadata
	req, _ := http.NewRequest("PUT", suite.Base+"/"+bucket+"/dst.txt", nil)
	req.Header.Set("X-Amz-Copy-Source", bucket+"/src.txt")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 200 {
		t.Fatalf("copy: %d", code)
	}
	req, _ = http.NewRequest("HEAD", suite.Base+"/"+bucket+"/dst.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("X-Amz-Meta-K") != "orig" {
		t.Fatalf("copy meta kept: %d %q", res.StatusCode, res.Header.Get("X-Amz-Meta-K"))
	}

	// REPLACE swaps metadata
	req, _ = http.NewRequest("PUT", suite.Base+"/"+bucket+"/dst.txt", nil)
	req.Header.Set("X-Amz-Copy-Source", bucket+"/src.txt")
	req.Header.Set("X-Amz-Metadata-Directive", "REPLACE")
	req.Header.Set("X-Amz-Meta-K", "novo")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 200 {
		t.Fatalf("copy replace: %d", code)
	}
	req, _ = http.NewRequest("HEAD", suite.Base+"/"+bucket+"/dst.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.Header.Get("X-Amz-Meta-K") != "novo" {
		t.Fatalf("copy meta replaced: %q", res.Header.Get("X-Amz-Meta-K"))
	}

	// missing source -> 404
	req, _ = http.NewRequest("PUT", suite.Base+"/"+bucket+"/dst2.txt", nil)
	req.Header.Set("X-Amz-Copy-Source", bucket+"/nope.txt")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 404 {
		t.Fatalf("copy missing: %d", code)
	}
}
