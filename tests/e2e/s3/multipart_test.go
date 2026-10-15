package s3

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"blob/tests/e2e/testutil"
)

func s3req(t *testing.T, method, url string, body io.Reader, clen int64) *http.Request {
	return s3reqType(t, method, url, body, clen, "application/octet-stream")
}

func s3reqType(t *testing.T, method, url string, body io.Reader, clen int64, ctype string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.ContentLength = clen
		req.Header.Set("Content-Type", ctype)
	}
	ph := testutil.EmptySHA256
	if body != nil {
		b, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
		}
		ph = testutil.Sha256hex(b)
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.ContentLength = int64(len(b))
	}
	testutil.SignV4(req, suite.S3ID, suite.S3Secret, "us-east-1", "s3", time.Now().UTC(), ph)
	return req
}

func TestMultipart(t *testing.T) {
	bucket := suite.Bucket(t, "mp")
	base := suite.Base + "/" + bucket + "/big.bin"

	// create
	req := s3req(t, "POST", base+"?uploads=", nil, 0)
	code, body := testutil.Do(t, req)
	if code != 200 || !strings.Contains(string(body), "<UploadId>") {
		t.Fatalf("create: %d %s", code, body)
	}
	type created struct {
		UploadID string `xml:"UploadId"`
	}
	var c created
	if err := xml.Unmarshal(body, &c); err != nil || c.UploadID == "" {
		t.Fatalf("create xml: %v %s", err, body)
	}
	defer func() {
		req := s3req(t, "DELETE", base+"?uploadId="+c.UploadID, nil, 0)
		testutil.Do(t, req)
	}()

	// 2 parts of 2MB
	part := bytes.Repeat([]byte("b"), 2*1024*1024)
	for i := 1; i <= 2; i++ {
		req := s3req(t, "PUT", base+"?partNumber="+strconv.Itoa(i)+"&uploadId="+c.UploadID, bytes.NewReader(part), int64(len(part)))
		if code, b := testutil.Do(t, req); code != 200 {
			t.Fatalf("part %d: %d %s", i, code, b)
		}
	}

	// list parts
	req = s3req(t, "GET", base+"?uploadId="+c.UploadID, nil, 0)
	if code, b := testutil.Do(t, req); code != 200 || strings.Count(string(b), "<PartNumber>") != 2 {
		t.Fatalf("list parts: %d %s", code, b)
	}

	// complete
	payload := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber></Part><Part><PartNumber>2</PartNumber></Part></CompleteMultipartUpload>`
	req = s3reqType(t, "POST", base+"?uploadId="+c.UploadID, strings.NewReader(payload), int64(len(payload)), "application/xml")
	if code, b := testutil.Do(t, req); code != 200 || !strings.Contains(string(b), "<ETag>") {
		t.Fatalf("complete: %d %s", code, b)
	}

	// bytes match
	req = s3req(t, "GET", base, nil, 0)
	code, b := testutil.Do(t, req)
	if code != 200 || len(b) != len(part)*2 {
		t.Fatalf("get after complete: %d len=%d", code, len(b))
	}
	s3delete(t, bucket, "big.bin")
}

func TestMultipartAbort(t *testing.T) {
	bucket := suite.Bucket(t, "mpabort")
	base := suite.Base + "/" + bucket + "/x.bin"
	req := s3req(t, "POST", base+"?uploads=", nil, 0)
	code, body := testutil.Do(t, req)
	if code != 200 {
		t.Fatalf("create: %d", code)
	}
	start := strings.Index(string(body), "<UploadId>")
	end := strings.Index(string(body), "</UploadId>")
	if start < 0 {
		t.Fatalf("no upload id: %s", body)
	}
	uid := string(body)[start+10 : end]

	// listed while in progress
	req = s3req(t, "GET", suite.Base+"/"+bucket+"?uploads", nil, 0)
	if code, b := testutil.Do(t, req); code != 200 || !strings.Contains(string(b), uid) {
		t.Fatalf("list uploads: %d %s", code, b)
	}

	req = s3req(t, "DELETE", base+"?uploadId="+uid, nil, 0)
	if code, _ := testutil.Do(t, req); code != 204 {
		t.Fatalf("abort: %d", code)
	}

	// completing after abort -> 404
	req = s3req(t, "POST", base+"?uploadId="+uid, strings.NewReader("<CompleteMultipartUpload/>"), int64(len("<CompleteMultipartUpload/>")))
	if code, _ := testutil.Do(t, req); code != 404 {
		t.Fatalf("complete after abort: %d", code)
	}
}
