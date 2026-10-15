package s3

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func TestPresigned(t *testing.T) {
	bucket := suite.Bucket(t, "presigned")
	content := "conteudo-presigned"
	body := content
	req, _ := http.NewRequest("PUT", suite.Base+"/"+bucket+"/f.txt", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "text/plain")
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), testutil.Sha256hex([]byte(body)))
	if code, _ := testutil.Do(t, req); code != 200 {
		t.Fatalf("put: %d", code)
	}
	defer s3delete(t, bucket, "f.txt")

	u, err := testutil.PresignV4(suite.Base+"/"+bucket+"/f.txt", suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	rb, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || string(rb) != content {
		t.Fatalf("presigned: %d %q", res.StatusCode, rb)
	}

	// tampered signature rejected
	res, err = http.Get(u + "x")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("tampered presigned accepted")
	}

	// expired rejected
	expired, err := testutil.PresignV4(suite.Base+"/"+bucket+"/f.txt", suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC().Add(-time.Hour), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.Get(expired)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("expired presigned accepted")
	}
}
