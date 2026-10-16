package auth

import (
	"strings"
)

// Permission identifies an allowed operation.
type Permission string

const (
	PermRead   Permission = "read"
	PermWrite  Permission = "write"
	PermDelete Permission = "delete"
	PermAll    Permission = "all"
	PermAdmin  Permission = "admin"
)

// APIKey is a token with an optional permission scope.
type APIKey struct {
	Token     string
	AccessKey string
	Perms     map[Permission]bool
}

// HasPerm reports whether the key allows the given operation.
func (k *APIKey) HasPerm(p Permission) bool {
	if k.Perms[PermAll] {
		return true
	}
	return k.Perms[p]
}

// parsePermissions converts a "read,write" list into a permission set.
func parsePermissions(raw string) map[Permission]bool {
	perms := make(map[Permission]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		perms[Permission(part)] = true
	}
	return perms
}

// defaultPermissions grants every operation.
func defaultPermissions() map[Permission]bool {
	return map[Permission]bool{PermAll: true}
}

// RequirePermission checks that the given key permits the operation.
// A nil key (public/unauthenticated) never passes.
func RequirePermission(key *APIKey, perms ...Permission) bool {
	if key == nil {
		return false
	}
	for _, p := range perms {
		if !key.HasPerm(p) {
			return false
		}
	}
	return true
}
