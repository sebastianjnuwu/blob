package s3

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func TestGet(t *testing.T) {
	bucket := suite.Bucket(t, "get")
	content := "conteudo-get"
	if code, _ := signedPut(t, bucket, "a.txt", content, "text/plain", map[string]string{"X-Amz-Meta-K": "v"}); code != 200 {
		t.Fatalf("put: %d", code)
	}
	defer s3delete(t, bucket, "a.txt")

	req, _ := http.NewRequest("GET", suite.Base+"/"+bucket+"/a.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	code, body := testutil.Do(t, req)
	if code != 200 || string(body) != content {
		t.Fatalf("get: %d %q", code, body)
	}

	// metadata round-trips as response headers
	req, _ = http.NewRequest("HEAD", suite.Base+"/"+bucket+"/a.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("head: %d", res.StatusCode)
	}
	if res.Header.Get("X-Amz-Meta-K") != "v" {
		t.Fatalf("meta header: %q", res.Header.Get("X-Amz-Meta-K"))
	}
	if res.Header.Get("ETag") == "" || res.Header.Get("Content-Length") != "12" {
		t.Fatalf("headers: etag=%q len=%q", res.Header.Get("ETag"), res.Header.Get("Content-Length"))
	}
}

func TestGetNotFound(t *testing.T) {
	bucket := suite.Bucket(t, "get404")
	req, _ := http.NewRequest("GET", suite.Base+"/"+bucket+"/nope.txt", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	if code, _ := testutil.Do(t, req); code != 404 {
		t.Fatalf("get missing: %d", code)
	}
}

func TestGetPrivateAnonymousDenied(t *testing.T) {
	bucket := suite.Bucket(t, "getpriv")
	if code, _ := signedPut(t, bucket, "p.txt", "priv", "", nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	defer s3delete(t, bucket, "p.txt")

	res, err := http.Get(suite.Base + "/" + bucket + "/p.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("anon get private: %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "xml") {
		t.Fatalf("error not xml: %q", res.Header.Get("Content-Type"))
	}
}
