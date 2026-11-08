package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSignAndVerifySignature(t *testing.T) {
	secret := "secret"
	id := "object-id"
	now := time.Unix(1700000000, 0)
	expires := now.Add(15 * time.Minute)
	sig := Sign(secret, id, ActionDownload, expires)
	if sig == "" {
		t.Fatal("expected signature")
	}
	if !VerifySignature(secret, id, ActionDownload, strconv.FormatInt(expires.Unix(), 10), sig, now) {
		t.Fatal("valid signature rejected")
	}
	if VerifySignature(secret, id, ActionView, strconv.FormatInt(expires.Unix(), 10), sig, now) {
		t.Fatal("signature leaked across action")
	}
	if VerifySignature(secret+"x", id, ActionDownload, strconv.FormatInt(expires.Unix(), 10), sig, now) {
		t.Fatal("signature leaked across secret")
	}
}

func TestVerifySignatureRejectsExpired(t *testing.T) {
	secret := "secret"
	id := "object-id"
	now := time.Unix(1700000000, 0)
	expires := now.Add(-time.Minute)
	sig := Sign(secret, id, ActionDownload, expires)
	if VerifySignature(secret, id, ActionDownload, strconv.FormatInt(expires.Unix(), 10), sig, now) {
		t.Fatal("expired signature accepted")
	}
}

func TestVerifySignatureRejectsMalformed(t *testing.T) {
	if VerifySignature("secret", "id", ActionDownload, "not-a-number", "sig", time.Now()) {
		t.Fatal("malformed expires accepted")
	}
	if VerifySignature("secret", "id", ActionDownload, fmt.Sprint(time.Now().Unix()), "", time.Now()) {
		t.Fatal("empty signature accepted")
	}
}

func TestDefaultTTLPositive(t *testing.T) {
	if got := DefaultTTL(); got <= 0 {
		t.Fatalf("expected positive ttl, got %v", got)
	}
}

func TestParseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/blob/x/download?id=obj&action=download&expires=123&signature=abc", nil)
	id, action, expires, signature := ParseURL(req)
	if id != "obj" || action != ActionDownload || expires != "123" || signature != "abc" {
		t.Fatalf("got %q %q %q %q", id, action, expires, signature)
	}
}
