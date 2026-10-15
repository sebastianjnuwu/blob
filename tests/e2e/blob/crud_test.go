package blob

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"testing"

	"blob/tests/e2e/testutil"
)

func TestUploadGetDelete(t *testing.T) {
	bucket := suite.Bucket(t, "crud")
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("bucket", bucket)
	_ = w.WriteField("filename", "nota.txt")
	fw, err := w.CreateFormFile("file", "nota.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("conteudo-crud"))
	w.Close()

	req := testutil.BearerReq(t, "PUT", suite.Base+"/blob", suite.Bearer, "", "")
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Body = io.NopCloser(&body)
	req.ContentLength = int64(body.Len())
	code, rb := testutil.Do(t, req)
	if code != 200 {
		t.Fatalf("upload: %d %s", code, rb)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rb, &created); err != nil || created.ID == "" {
		t.Fatalf("upload decode: %v %s", err, rb)
	}

	if code, _ := testutil.Do(t, testutil.BearerReq(t, "GET", suite.Base+"/blob/"+created.ID, suite.Bearer, "", "")); code != 200 {
		t.Fatalf("get meta: %d", code)
	}
	if code, data := testutil.Do(t, testutil.BearerReq(t, "GET", suite.Base+"/blob/"+created.ID+"/download", suite.Bearer, "", "")); code != 200 || string(data) != "conteudo-crud" {
		t.Fatalf("download: %d %q", code, data)
	}
	if code, _ := testutil.Do(t, testutil.BearerReq(t, "DELETE", suite.Base+"/blob/"+created.ID, suite.Bearer, "", "")); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := testutil.Do(t, testutil.BearerReq(t, "GET", suite.Base+"/blob/"+created.ID, suite.Bearer, "", "")); code != 404 {
		t.Fatalf("get after delete: %d", code)
	}
}
