package auth

import (
	"github.com/google/uuid"
	"net/http/httptest"
	"testing"
)

func TestStableUserIDIsDeterministic(t *testing.T) {
	a := StableUserID("token-a")
	b := StableUserID("token-a")
	if a != b {
		t.Fatalf("expected stable id, got %v then %v", a, b)
	}
	if c := StableUserID("token-b"); c == a {
		t.Fatal("expected different ids for different credentials")
	}
}

func TestBearerFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer abc123")
	if got := bearerFromRequest(req); got != "abc123" {
		t.Fatalf("got %q", got)
	}

	req.Header.Set("Authorization", "Basic nope")
	if got := bearerFromRequest(req); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestUserIDFromRequestWithoutDB(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	if got := UserIDFromRequest(req); got != uuid.Nil {
		t.Fatalf("expected nil uuid without db, got %v", got)
	}
}
