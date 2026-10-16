package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blob/src/auth"
	"blob/src/config"
	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/services"
)

// requestBase builds the public base URL of the server. BLOB_PUBLIC_URL takes
// precedence; forwarded headers are honored only when the proxy is trusted.
func requestBase(r *http.Request) string {
	if config.Env != nil && config.Env.PublicURL != "" {
		return strings.TrimRight(config.Env.PublicURL, "/")
	}

	proto := "http"
	host := r.Host
	if config.Env != nil && config.Env.TrustProxy {
		if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
			proto = strings.TrimSpace(strings.Split(p, ",")[0])
		}
		if h := r.Header.Get("X-Forwarded-Host"); h != "" {
			host = strings.TrimSpace(strings.Split(h, ",")[0])
		}
	}
	if proto == "" {
		proto = "http"
	}
	return proto + "://" + host
}

// SignBlobController handles POST /blob/{id}/sign (private).
// It returns expiring download and view URLs signed with HMAC-SHA256.
func SignBlobController(w http.ResponseWriter, r *http.Request) {
	id, ok := parseBlobID(r)
	if !ok {
		functions.WriteJSONError(w, "Invalid blob id", http.StatusBadRequest)
		return
	}

	if _, err := services.Blobs.Get(r.Context(), id); err != nil {
		functions.WriteJSONError(w, "Blob not found", http.StatusNotFound)
		return
	}

	expires := time.Now().UTC().Add(auth.DefaultTTL())
	expiresUnix := expires.Unix()

	base := requestBase(r)

	downloadSig := auth.Sign(auth.Secret(), id.String(), auth.ActionDownload, expires)
	viewSig := auth.Sign(auth.Secret(), id.String(), auth.ActionView, expires)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         id.String(),
		"expires_at": expires.Format(time.RFC3339),
		"ttl":        int64(auth.DefaultTTL().Seconds()),
		"download":   base + "/blob/" + id.String() + "/download?expires=" + strconv.FormatInt(expiresUnix, 10) + "&signature=" + downloadSig,
		"view":       base + "/blob/" + id.String() + "/view?expires=" + strconv.FormatInt(expiresUnix, 10) + "&signature=" + viewSig,
	}); err != nil {
		logger.Error("failed to encode sign json: %v", err)
	}
}
