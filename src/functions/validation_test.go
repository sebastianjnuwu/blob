package functions

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStringInSlice(t *testing.T) {
	if !StringInSlice("b", []string{"a", "b", "c"}) {
		t.Fatal("expected true")
	}
	if StringInSlice("z", []string{"a", "b"}) {
		t.Fatal("expected false")
	}
}

func TestSplitAndTrimAndSplitComma(t *testing.T) {
	got := SplitAndTrim(" a , , b ,c ", ",")
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
	if comma := SplitComma(""); comma != nil {
		t.Fatalf("expected nil, got %v", comma)
	}
}

func TestIsAllowedMimeType(t *testing.T) {
	if !IsAllowedMimeType("image/png", []string{"*"}) {
		t.Fatal("wildcard should allow")
	}
	if !IsAllowedMimeType("image/png", []string{"image/png"}) {
		t.Fatal("exact should allow")
	}
	if IsAllowedMimeType("image/png", []string{"text/plain"}) {
		t.Fatal("unexpected allow")
	}
}

func TestGetTotalStorageSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abcd"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sub := filepath.Join(dir, "sub"); true {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("xy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	total, err := GetTotalStorageSize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if total != 6 {
		t.Fatalf("got %d, want 6", total)
	}
}

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSONError(rr, "boom", http.StatusBadRequest)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
}
