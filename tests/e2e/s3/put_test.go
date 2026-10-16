package s3

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func signedPut(t *testing.T, bucket, key, content, ctype string, extra map[string]string) (int, http.Header) {
	t.Helper()
	body := strings.NewReader(content)
	req, err := http.NewRequest("PUT", suite.Base+"/"+bucket+"/"+key, body)
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = int64(len(content))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.Sha256hex([]byte(content)))
	code, _ := testutil.Do(t, req)
	return code, req.Header
}

func s3delete(t *testing.T, bucket, key string) {
	t.Helper()
	req, _ := http.NewRequest("DELETE", suite.Base+"/"+bucket+"/"+key, nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	code, _ := testutil.Do(t, req)
	if code != 204 {
		t.Fatalf("cleanup delete %s: %d", key, code)
	}
}

func TestPut(t *testing.T) {
	bucket := suite.Bucket(t, "put")
	req, _ := http.NewRequest("PUT", suite.Base+"/"+bucket+"/a.txt", strings.NewReader("hello-put"))
	req.ContentLength = 9
	req.Header.Set("Content-Type", "text/plain")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.Sha256hex([]byte("hello-put")))
	code, _ := testutil.Do(t, req)
	if code != 200 {
		t.Fatalf("put: %d", code)
	}

	// overwrite same key replaces instead of duplicating
	req, _ = http.NewRequest("PUT", suite.Base+"/"+bucket+"/a.txt", strings.NewReader("hello-put"))
	req.ContentLength = 9
	req.Header.Set("Content-Type", "text/plain")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.Sha256hex([]byte("hello-put")))
	code, _ = testutil.Do(t, req)
	if code != 200 {
		t.Fatalf("overwrite: %d", code)
	}

	// traversal rejected
	req, _ = http.NewRequest("PUT", suite.Base+"/"+bucket+"/../x", strings.NewReader("x"))
	req.ContentLength = 1
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.Sha256hex([]byte("x")))
	code, _ = testutil.Do(t, req)
	if code == 200 {
		t.Fatal("traversal accepted")
	}

	signedPut(t, bucket, "cleanup.txt", "x", "", nil)
	defer s3delete(t, bucket, "a.txt")
	defer s3delete(t, bucket, "cleanup.txt")
}

func TestPutPublicACL(t *testing.T) {
	bucket := suite.Bucket(t, "acl")
	if code, _ := signedPut(t, bucket, "pub.txt", "public-data", "text/plain", map[string]string{"X-Amz-Acl": "public-read"}); code != 200 {
		t.Fatalf("put public: %d", code)
	}
	defer s3delete(t, bucket, "pub.txt")
	// anonymous GET works on public objects
	res, err := http.Get(suite.Base + "/" + bucket + "/pub.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("anon get public: %d", res.StatusCode)
	}
}
