package auth

import (
	"net/http"
	"testing"
	"time"
)

// Signed with @smithy/signature-v4 (us-east-1/s3, 2025-01-01T00:00:00Z):
// GET http://127.0.0.1:3099/blob/abc?list-type=2&prefix=a%20b
// secret: abcdefghijklmnopqrstuvwxyz0123456789ABCD
func TestVerifyS3SmithyVector(t *testing.T) {
	r, err := http.NewRequest("GET", "http://127.0.0.1:3099/blob/abc?list-type=2&prefix=a%20b", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Host = "127.0.0.1:3099"
	r.Header.Set("X-Amz-Date", "20250101T000000Z")
	r.Header.Set("X-Amz-Content-Sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY1234567890AB/20250101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=e8d88031e0fb63e147e40fd142bb4daf355e14d7e58150986064af7cb1c98e9d")

	lookup := func(akid string) (string, map[Permission]bool, bool) {
		if akid != "TESTKEY1234567890AB" {
			return "", nil, false
		}
		return "abcdefghijklmnopqrstuvwxyz0123456789ABCD", map[Permission]bool{PermRead: true}, true
	}
	now, _ := time.Parse("20060102T150405Z", "20250101T000000Z")
	key := verifyS3(r, now, lookup)
	if key == nil {
		t.Fatal("smithy signature rejected")
	}
	if !key.HasPerm(PermRead) || key.HasPerm(PermWrite) {
		t.Fatal("wrong permissions")
	}
}
