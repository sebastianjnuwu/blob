package auth

import "testing"

func TestParsePermissions(t *testing.T) {
	got := parsePermissions(" read, write , ,delete,ALL ")
	wants := []Permission{PermRead, PermWrite, PermDelete, PermAll}
	for _, p := range wants {
		if !got[p] {
			t.Fatalf("missing permission %q in %v", p, got)
		}
	}
	if len(got) != len(wants) {
		t.Fatalf("got %d permissions, want %d", len(got), len(wants))
	}
}

func TestHasPerm(t *testing.T) {
	key := &APIKey{Perms: map[Permission]bool{PermRead: true}}
	if !key.HasPerm(PermRead) {
		t.Fatal("expected read")
	}
	if key.HasPerm(PermWrite) {
		t.Fatal("unexpected write")
	}

	all := &APIKey{Perms: parsePermissions("all")}
	if !all.HasPerm(PermDelete) {
		t.Fatal("all should grant delete")
	}
}

func TestRequirePermission(t *testing.T) {
	if RequirePermission(nil, PermRead) {
		t.Fatal("nil key must fail")
	}

	readOnly := &APIKey{Perms: map[Permission]bool{PermRead: true}}
	if !RequirePermission(readOnly, PermRead) {
		t.Fatal("read key should pass read")
	}
	if RequirePermission(readOnly, PermRead, PermWrite) {
		t.Fatal("read key must fail write")
	}
}

func TestDefaultPermissions(t *testing.T) {
	p := defaultPermissions()
	if !p[PermAll] {
		t.Fatalf("default permissions must include all")
	}
}
