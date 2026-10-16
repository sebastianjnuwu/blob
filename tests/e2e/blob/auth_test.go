package blob

import (
	"net/http"
	"testing"

	"blob/tests/e2e/testutil"
)

func TestHealth(t *testing.T) {
	res, err := http.Get(suite.Base + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("health: %d", res.StatusCode)
	}
}

func TestAuthGuards(t *testing.T) {
	res, err := http.Get(suite.Base + "/blob")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("no key: got %d, want 401", res.StatusCode)
	}

	req := testutil.BearerReq(t, "GET", suite.Base+"/blob", "wrong", "", "")
	if code, _ := testutil.Do(t, req); code != 401 {
		t.Fatalf("bad key: got %d, want 401", code)
	}

	req = testutil.BearerReq(t, "PUT", suite.Base+"/blob", suite.ReadKey, "x", "")
	req.ContentLength = 1
	if code, _ := testutil.Do(t, req); code != 403 {
		t.Fatalf("read key writing: got %d, want 403", code)
	}
}
