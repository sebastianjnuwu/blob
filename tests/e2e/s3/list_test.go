package s3

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func TestListBuckets(t *testing.T) {
	bucket := suite.Bucket(t, "listb")
	if code, _ := signedPut(t, bucket, "a.txt", "x", "", nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	defer s3delete(t, bucket, "a.txt")

	req, _ := http.NewRequest("GET", suite.Base+"/", nil)
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
	code, body := testutil.Do(t, req)
	if code != 200 || !strings.Contains(string(body), "<Name>"+bucket+"</Name>") {
		t.Fatalf("list buckets: %d %s", code, body)
	}
}

func TestListObjects(t *testing.T) {
	bucket := suite.Bucket(t, "listo")
	for _, k := range []string{"doc/a.txt", "doc/b.txt", "img/c.png"} {
		if code, _ := signedPut(t, bucket, k, "x", "", nil); code != 200 {
			t.Fatalf("put %s: %d", k, code)
		}
		defer s3delete(t, bucket, k)
	}

	list := func(query string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest("GET", suite.Base+"/"+bucket+"?list-type=2&"+query, nil)
		testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.EmptySHA256)
		code, body := testutil.Do(t, req)
		return code, string(body)
	}

	if code, body := list("prefix=doc/"); code != 200 || strings.Count(body, "<Key>") != 2 {
		t.Fatalf("prefix: %d %s", code, body)
	}
	if code, body := list("max-keys=1"); code != 200 || strings.Count(body, "<Key>") != 1 || !strings.Contains(body, "<IsTruncated>true</IsTruncated>") {
		t.Fatalf("max-keys: %d %s", code, body)
	}
	// pagination via continuation token
	_, paged := list("max-keys=1")
	body := string(paged)
	start := strings.Index(body, "<NextContinuationToken>")
	token := ""
	if start >= 0 {
		rest := body[start+len("<NextContinuationToken>"):]
		if end := strings.Index(rest, "</NextContinuationToken>"); end > 0 {
			token = rest[:end]
		}
	}
	if token == "" {
		t.Fatalf("no continuation token: %s", body)
	}
	if code, body := list("max-keys=1&continuation-token=" + token); code != 200 || !strings.Contains(body, "doc/b.txt") {
		t.Fatalf("page 2: %d %s", code, body)
	}
}
