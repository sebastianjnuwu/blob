package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// sigV4Prefix is the Authorization scheme for AWS Signature Version 4.
const sigV4Prefix = "AWS4-HMAC-SHA256 "

// VerifyS3Request verifies an AWS SigV4-signed request and returns the key
// when the signature matches an active access key. now is injected for tests.
func VerifyS3Request(r *http.Request, now time.Time) *APIKey {
	return verifyS3(r, now, S3Credentials)
}

func verifyS3(r *http.Request, now time.Time, lookup func(string) (string, map[Permission]bool, bool)) *APIKey {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, sigV4Prefix) {
		return nil
	}
	credential, signedHeaders, signature := parseSigV4Header(strings.TrimPrefix(h, sigV4Prefix))
	if credential == "" || signedHeaders == "" || signature == "" {
		return nil
	}
	scope := strings.Split(credential, "/")
	if len(scope) != 5 || scope[4] != "aws4_request" {
		return nil
	}
	accessKey, datestamp, region, service := scope[0], scope[1], scope[2], scope[3]

	secret, perms, ok := lookup(accessKey)
	if !ok {
		return nil
	}

	amzDate := r.Header.Get("X-Amz-Date")
	t, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return nil
	}
	if skew := now.Sub(t); skew > 15*time.Minute || skew < -15*time.Minute {
		return nil
	}

	payload := r.Header.Get("X-Amz-Content-Sha256")
	if payload == "" {
		return nil
	}

	names := strings.Split(strings.ToLower(signedHeaders), ";")
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		var v string
		if n == "host" {
			v = r.Host
		} else {
			v = r.Header.Get(http.CanonicalHeaderKey(n))
		}
		if v == "" {
			return nil
		}
		lines = append(lines, n+":"+strings.Join(strings.Fields(v), " "))
	}

	canonical := strings.Join([]string{
		r.Method,
		encodeURI(r.URL.EscapedPath()),
		encodeQuery(r.URL.Query()),
		strings.Join(lines, "\n") + "\n",
		strings.Join(names, ";"),
		payload,
	}, "\n")

	scopeStr := strings.Join(scope[1:4], "/") + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scopeStr + "\n" + hex.EncodeToString(sum[:])

	k := signingKey(secret, datestamp, region, service)
	if !checkSignature(k, toSign, signature) {
		return nil
	}
	return &APIKey{Token: "s3:" + accessKey, AccessKey: accessKey, Perms: perms}
}

// VerifyS3 verifies with the current time.
func VerifyS3(r *http.Request) *APIKey { return VerifyS3Request(r, time.Now().UTC()) }

func parseSigV4Header(h string) (credential, signedHeaders, signature string) {
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "Credential="):
			credential = strings.TrimPrefix(part, "Credential=")
		case strings.HasPrefix(part, "SignedHeaders="):
			signedHeaders = strings.TrimPrefix(part, "SignedHeaders=")
		case strings.HasPrefix(part, "Signature="):
			signature = strings.TrimPrefix(part, "Signature=")
		}
	}
	return credential, signedHeaders, signature
}

func hmacSHA256(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// checkSignature recomputes the SigV4 signature and compares in constant time.
func checkSignature(signingKey []byte, toSign, signatureHex string) bool {
	expected := hex.EncodeToString(hmacSHA256(signingKey, toSign))
	sig, err := hex.DecodeString(strings.ToLower(strings.TrimSpace(signatureHex)))
	if err != nil {
		return false
	}
	exp, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(sig, exp) == 1
}

// signingKey derives the SigV4 signing key.
func signingKey(secret, datestamp, region, service string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), datestamp)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	return hmacSHA256(k, "aws4_request")
}

// VerifyS3Presigned verifies a presigned URL (SigV4 in query string).
func VerifyS3Presigned(r *http.Request, now time.Time) *APIKey {
	q := r.URL.Query()
	if q.Get("X-Amz-Signature") == "" {
		return nil
	}
	return verifyS3Presigned(r, now, S3Credentials, q)
}

func verifyS3Presigned(r *http.Request, now time.Time, lookup func(string) (string, map[Permission]bool, bool), q url.Values) *APIKey {
	credential, signedHeaders, signature := q.Get("X-Amz-Credential"), q.Get("X-Amz-SignedHeaders"), q.Get("X-Amz-Signature")
	amzDate, expires := q.Get("X-Amz-Date"), q.Get("X-Amz-Expires")
	if credential == "" || signedHeaders == "" || signature == "" || amzDate == "" || expires == "" {
		return nil
	}
	scope := strings.Split(credential, "/")
	if len(scope) != 5 || scope[4] != "aws4_request" {
		return nil
	}
	accessKey, datestamp, region, service := scope[0], scope[1], scope[2], scope[3]
	secret, perms, ok := lookup(accessKey)
	if !ok {
		return nil
	}
	t, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return nil
	}
	ttl, err := strconv.Atoi(expires)
	if err != nil || ttl <= 0 {
		return nil
	}
	if now.After(t.Add(time.Duration(ttl) * time.Second)) {
		return nil
	}

	filtered := url.Values{}
	for k, vs := range q {
		if strings.EqualFold(k, "X-Amz-Signature") {
			continue
		}
		filtered[k] = vs
	}
	names := strings.Split(strings.ToLower(signedHeaders), ";")
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		var v string
		if n == "host" {
			v = r.Host
		} else {
			v = r.Header.Get(http.CanonicalHeaderKey(n))
		}
		if v == "" {
			return nil
		}
		lines = append(lines, n+":"+strings.Join(strings.Fields(v), " "))
	}
	canonical := strings.Join([]string{
		r.Method,
		encodeURI(r.URL.EscapedPath()),
		encodeQuery(filtered),
		strings.Join(lines, "\n") + "\n",
		strings.Join(names, ";"),
		"UNSIGNED-PAYLOAD",
	}, "\n")
	scopeStr := strings.Join(scope[1:4], "/") + "/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scopeStr + "\n" + hex.EncodeToString(sum[:])
	if !checkSignature(signingKey(secret, datestamp, region, service), toSign, signature) {
		return nil
	}
	return &APIKey{Token: "s3:" + accessKey, AccessKey: accessKey, Perms: perms}
}

// encodeURI normalizes an escaped path: keeps '/' and valid %XX triplets,
// percent-encodes anything else outside the unreserved set.
func encodeURI(escaped string) string {
	if escaped == "" {
		return "/"
	}
	const hexd = "0123456789ABCDEF"
	isHex := func(c byte) bool {
		return c >= '0' && c <= '9' || c >= 'A' && c <= 'F' || c >= 'a' && c <= 'f'
	}
	var b strings.Builder
	for i := 0; i < len(escaped); i++ {
		c := escaped[i]
		switch {
		case c == '/':
			b.WriteByte(c)
		case c == '%' && i+2 < len(escaped) && isHex(escaped[i+1]) && isHex(escaped[i+2]):
			b.WriteString(escaped[i : i+3])
			i += 2
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexd[c>>4])
			b.WriteByte(hexd[c&15])
		}
	}
	return b.String()
}

// encodeQuery canonicalizes query params (sorted, SigV4-escaped).
func encodeQuery(q url.Values) string {
	type kv struct{ k, v string }
	var pairs []kv
	for k, vs := range q {
		for _, v := range vs {
			pairs = append(pairs, kv{escapeSigV4(k), escapeSigV4(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k == pairs[j].k {
			return pairs[i].v < pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(p.k)
		b.WriteByte('=')
		b.WriteString(p.v)
	}
	return b.String()
}

func escapeSigV4(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteByte(c)
		default:
			const hexd = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hexd[c>>4])
			b.WriteByte(hexd[c&15])
		}
	}
	return b.String()
}
