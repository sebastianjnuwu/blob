package controllers

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"blob/src/auth"
	"blob/src/config"
	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/repository"
	"blob/src/services"

	"github.com/google/uuid"
)

// S3 compatibility shim: bucket ops + object ops over the blob service.
// Auth accepts bearer tokens or SigV4 signatures; errors follow S3 XML.

type s3Error struct {
	Status  int
	Code    string
	Message string
}

func writeS3Error(w http.ResponseWriter, e s3Error) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(e.Status)
	_ = xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"Error"`
		Code    string   `xml:"Code"`
		Message string   `xml:"Message"`
	}{Code: e.Code, Message: e.Message})
}

// s3Authorize returns the key for bearer or SigV4 credentials or writes the error.
func s3Authorize(w http.ResponseWriter, r *http.Request, perm auth.Permission) *auth.APIKey {
	key := auth.Verify(bearerTokenOf(r))
	if key == nil {
		key = auth.VerifyS3(r)
	}
	if key == nil {
		writeS3Error(w, s3Error{http.StatusForbidden, "SignatureDoesNotMatch", "The request signature does not match."})
		return nil
	}
	if !auth.RequirePermission(key, perm) {
		writeS3Error(w, s3Error{http.StatusForbidden, "AccessDenied", "Insufficient permissions."})
		return nil
	}
	return key
}

func bearerTokenOf(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(h[7:])
}

// etagOf returns the quoted content hash (sha256-based, opaque to clients).
func etagOf(hash string) string { return `"` + hash + `"` }

// splitS3Path splits "/bucket/key..." into bucket and key.
func splitS3Path(p string) (bucket, key string) {
	p = strings.Trim(p, "/")
	if p == "" {
		return "", ""
	}
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i], p[i+1:]
	}
	return p, ""
}

// validS3Key rejects traversal and empty keys.
func validS3Key(key string) bool {
	if key == "" || len(key) > 1024 || strings.HasPrefix(key, "/") {
		return false
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// S3Handler dispatches S3-compatible operations (fallback route).
func S3Handler(w http.ResponseWriter, r *http.Request) {
	bucket, key := splitS3Path(r.URL.Path)
	q := r.URL.Query()

	switch r.Method {
	case "PUT":
		if key == "" {
			s3CreateBucket(w, r, bucket)
			return
		}
		s3PutObject(w, r, bucket, key)
	case "GET":
		if _, ok := q["list-type"]; ok {
			s3ListObjects(w, r, bucket)
			return
		}
		if bucket == "" {
			s3ListBuckets(w, r)
			return
		}
		if key == "" {
			writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Missing key."})
			return
		}
		s3GetObject(w, r, bucket, key)
	case "DELETE":
		if key == "" {
			s3DeleteBucket(w, r, bucket)
			return
		}
		s3DeleteObject(w, r, bucket, key)
	case "HEAD":
		if key == "" {
			s3HeadBucket(w, r, bucket)
			return
		}
		s3HeadObject(w, r, bucket, key)
	default:
		writeS3Error(w, s3Error{http.StatusMethodNotAllowed, "MethodNotAllowed", "Method not allowed."})
	}
}

func s3PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if s3Authorize(w, r, auth.PermWrite) == nil {
		return
	}
	if src := r.Header.Get("X-Amz-Copy-Source"); src != "" {
		s3CopyObject(w, r, bucket, key, src)
		return
	}
	if bucket == "" || !validS3Key(key) {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Invalid bucket or key."})
		return
	}
	if r.ContentLength < 0 {
		writeS3Error(w, s3Error{http.StatusLengthRequired, "MissingContentLength", "Content-Length is required."})
		return
	}
	cfg := config.Env
	if cfg == nil {
		cfg = config.Load()
	}
	if cfg.MaxUploadSize > 0 && r.ContentLength > cfg.MaxUploadSize {
		writeS3Error(w, s3Error{http.StatusBadRequest, "EntityTooLarge", "Object too large."})
		return
	}
	if total, err := functions.GetTotalStorageSize(cfg.ResolveStoragePath()); err == nil && cfg.MaxStorageSize > 0 && total+r.ContentLength > cfg.MaxStorageSize {
		writeS3Error(w, s3Error{http.StatusInsufficientStorage, "QuotaExceeded", "Storage limit exceeded."})
		return
	}

	mime := r.Header.Get("Content-Type")
	var body io.Reader = r.Body
	if mime == "" {
		buf := make([]byte, 512)
		n, _ := io.ReadFull(r.Body, buf)
		mime = http.DetectContentType(buf[:n])
		body = io.MultiReader(strings.NewReader(string(buf[:n])), r.Body)
	}
	if !functions.IsAllowedMimeType(mime, cfg.AllowedMimeTypes) {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "MIME type not allowed."})
		return
	}

	meta := map[string]any{}
	for name, vs := range r.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-meta-") && len(vs) > 0 {
			meta[strings.ToLower(name[len("X-Amz-Meta-"):])] = vs[0]
		}
	}
	if len(meta) == 0 {
		meta = nil
	}

	// S3 overwrites same-key objects: upload first (dedup may share the
	// file), then drop the previous row if it is a different object.
	var prevID uuid.UUID
	hasPrev := false
	if prev, err := services.Blobs.GetByBucketFilename(r.Context(), bucket, key); err == nil {
		prevID, hasPrev = prev.ID, true
	}

	blob, err := services.Blobs.Upload(r.Context(), services.UploadInput{
		Bucket: bucket, Filename: key, Mime: mime, Size: r.ContentLength,
		Public: false, Metadata: meta, File: body,
	})
	if err != nil {
		logger.Error("s3 put %s/%s: %v", bucket, key, err)
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to store object."})
		return
	}
	if hasPrev && prevID != blob.ID {
		_ = services.Blobs.Delete(r.Context(), prevID)
	}
	w.Header().Set("ETag", etagOf(blob.Hash))
	w.WriteHeader(http.StatusOK)
}

func s3GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if s3Authorize(w, r, auth.PermRead) == nil {
		return
	}
	blob, err := services.Blobs.GetByBucketFilename(r.Context(), bucket, key)
	if err != nil {
		writeS3Error(w, s3Error{http.StatusNotFound, "NoSuchKey", "Object not found."})
		return
	}
	f, err := services.Blobs.OpenFile(r.Context(), blob.ID, "download_count")
	if err != nil {
		if os.IsNotExist(err) {
			writeS3Error(w, s3Error{http.StatusNotFound, "NoSuchKey", "Object not found."})
			return
		}
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to open object."})
		return
	}
	defer f.Reader.Close()
	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(f.Size, 10))
	w.Header().Set("ETag", etagOf(blob.Hash))
	w.Header().Set("Last-Modified", blob.UpdatedAt.UTC().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, key, blob.UpdatedAt, f.Reader)
}

func s3HeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if s3Authorize(w, r, auth.PermRead) == nil {
		return
	}
	blob, err := services.Blobs.GetByBucketFilename(r.Context(), bucket, key)
	if err != nil {
		writeS3Error(w, s3Error{http.StatusNotFound, "NoSuchKey", "Object not found."})
		return
	}
	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))
	w.Header().Set("ETag", etagOf(blob.Hash))
	w.Header().Set("Last-Modified", blob.UpdatedAt.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

func s3DeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if s3Authorize(w, r, auth.PermDelete) == nil {
		return
	}
	blob, err := services.Blobs.GetByBucketFilename(r.Context(), bucket, key)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := services.Blobs.Delete(r.Context(), blob.ID); err != nil {
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to delete object."})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// s3CopyObject implements S3 CopyObject (PUT + X-Amz-Copy-Source).
// The new row shares the physical file; refcounting handles deletion.
func s3CopyObject(w http.ResponseWriter, r *http.Request, bucket, key, src string) {
	if bucket == "" || !validS3Key(key) {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Invalid bucket or key."})
		return
	}
	srcPath, err := url.PathUnescape(strings.TrimPrefix(src, "/"))
	if err != nil {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Invalid copy source."})
		return
	}
	srcBucket, srcKey := splitS3Path("/" + srcPath)
	if srcBucket == "" || srcKey == "" {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Invalid copy source."})
		return
	}
	orig, err := services.Blobs.GetByBucketFilename(r.Context(), srcBucket, srcKey)
	if err != nil {
		writeS3Error(w, s3Error{http.StatusNotFound, "NoSuchKey", "Copy source not found."})
		return
	}
	dup := *orig
	dup.ID = uuid.New()
	dup.Bucket = bucket
	dup.Filename = key
	dup.CreatedAt = time.Now()
	dup.UpdatedAt = time.Now()
	if err := services.Blobs.Clone(r.Context(), &dup); err != nil {
		logger.Error("s3 copy %s/%s: %v", bucket, key, err)
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to copy object."})
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_ = xml.NewEncoder(w).Encode(struct {
		XMLName      xml.Name `xml:"CopyObjectResult"`
		ETag         string   `xml:"ETag"`
		LastModified string   `xml:"LastModified"`
	}{ETag: etagOf(dup.Hash), LastModified: dup.UpdatedAt.UTC().Format(time.RFC3339)})
}

func validBucket(bucket string) bool {
	if len(bucket) < 1 || len(bucket) > 63 {
		return false
	}
	for _, c := range bucket {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func s3CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if s3Authorize(w, r, auth.PermWrite) == nil {
		return
	}
	if !validBucket(bucket) {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidBucketName", "Invalid bucket name."})
		return
	}
	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

func s3HeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if s3Authorize(w, r, auth.PermRead) == nil {
		return
	}
	if bucket == "" {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Missing bucket."})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func s3DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if s3Authorize(w, r, auth.PermDelete) == nil {
		return
	}
	if bucket == "" {
		writeS3Error(w, s3Error{http.StatusBadRequest, "InvalidRequest", "Missing bucket."})
		return
	}
	res, err := services.Blobs.List(r.Context(), repository.ListFilter{Bucket: bucket, Page: 1, PageSize: 1})
	if err != nil {
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to check bucket."})
		return
	}
	if res.Total > 0 {
		writeS3Error(w, s3Error{http.StatusConflict, "BucketNotEmpty", "Bucket is not empty."})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func s3ListBuckets(w http.ResponseWriter, r *http.Request) {
	if s3Authorize(w, r, auth.PermRead) == nil {
		return
	}
	buckets, err := services.Blobs.Buckets(r.Context())
	if err != nil {
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to list buckets."})
		return
	}
	type outBucket struct {
		Name         string `xml:"Name"`
		CreationDate string `xml:"CreationDate"`
	}
	items := make([]outBucket, 0, len(buckets))
	for _, b := range buckets {
		items = append(items, outBucket{Name: b.Name, CreationDate: b.CreatedAt.UTC().Format(time.RFC3339)})
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_ = xml.NewEncoder(w).Encode(struct {
		XMLName xml.Name `xml:"ListAllMyBucketsResult"`
		Xmlns   string   `xml:"xmlns,attr"`
		Owner   struct {
			ID string `xml:"ID"`
		} `xml:"Owner"`
		Buckets struct {
			Items []outBucket `xml:"Bucket"`
		} `xml:"Buckets"`
	}{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Owner: struct {
			ID string `xml:"ID"`
		}{ID: "blob"},
		Buckets: struct {
			Items []outBucket `xml:"Bucket"`
		}{Items: items},
	})
}

func s3ListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	if s3Authorize(w, r, auth.PermRead) == nil {
		return
	}
	q := r.URL.Query()
	prefix := q.Get("prefix")
	maxKeys := 1000
	if v, err := strconv.Atoi(q.Get("max-keys")); err == nil && v > 0 {
		maxKeys = v
		if maxKeys > 1000 {
			maxKeys = 1000
		}
	}
	startAfter := q.Get("start-after")
	offset := 0
	if tok := q.Get("continuation-token"); tok != "" {
		if raw, err := base64.URLEncoding.DecodeString(tok); err == nil {
			if n, err := strconv.Atoi(string(raw)); err == nil && n > 0 {
				offset = n
			}
		}
	}
	encodeURL := q.Get("encoding-type") == "url"

	res, err := services.Blobs.List(r.Context(), repository.ListFilter{
		Bucket: bucket, Prefix: prefix, OrderKey: true,
		Page: offset/maxKeys + 1, PageSize: maxKeys,
	})
	if err != nil {
		writeS3Error(w, s3Error{http.StatusInternalServerError, "InternalError", "Failed to list objects."})
		return
	}
	rows := res.Blobs
	if skip := offset % maxKeys; skip > 0 && skip < len(rows) {
		rows = rows[skip:]
	}
	if startAfter != "" {
		kept := rows[:0]
		for _, b := range rows {
			if b.Filename > startAfter {
				kept = append(kept, b)
			}
		}
		rows = kept
	}

	type content struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}
	items := make([]content, 0, len(rows))
	for _, b := range rows {
		k := b.Filename
		if encodeURL {
			k = url.QueryEscape(k)
		}
		items = append(items, content{
			Key: k, LastModified: b.UpdatedAt.UTC().Format(time.RFC3339),
			ETag: etagOf(b.Hash), Size: b.Size, StorageClass: "STANDARD",
		})
	}
	truncated := offset+len(rows) < int(res.Total)
	nextToken := ""
	if truncated {
		nextToken = base64.URLEncoding.EncodeToString([]byte(strconv.Itoa(offset + len(rows))))
	}
	type result struct {
		XMLName               xml.Name  `xml:"ListBucketResult"`
		Xmlns                 string    `xml:"xmlns,attr"`
		Name                  string    `xml:"Name"`
		Prefix                string    `xml:"Prefix"`
		KeyCount              int       `xml:"KeyCount"`
		MaxKeys               int       `xml:"MaxKeys"`
		IsTruncated           bool      `xml:"IsTruncated"`
		Contents              []content `xml:"Contents"`
		EncodingType          string    `xml:"EncodingType,omitempty"`
		ContinuationToken     string    `xml:"ContinuationToken,omitempty"`
		NextContinuationToken string    `xml:"NextContinuationToken,omitempty"`
		StartAfter            string    `xml:"StartAfter,omitempty"`
	}
	out := result{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:  bucket, Prefix: prefix, KeyCount: len(items), MaxKeys: maxKeys,
		IsTruncated: truncated,
		Contents:    items,
	}
	if encodeURL {
		out.EncodingType = "url"
	}
	if tok := q.Get("continuation-token"); tok != "" {
		out.ContinuationToken = tok
	}
	if truncated {
		out.NextContinuationToken = nextToken
	}
	if startAfter != "" {
		out.StartAfter = startAfter
	}
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_ = xml.NewEncoder(w).Encode(out)
}
