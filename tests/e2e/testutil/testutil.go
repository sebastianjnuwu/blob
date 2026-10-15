// Package testutil boots the live e2e environment: it checks the server,
// provisions API keys via the CLI and revokes them afterwards.
package testutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Ctx carries one suite's credentials.
type Ctx struct {
	Base     string
	Bearer   string
	ReadKey  string
	S3ID     string
	S3Secret string
	label    string
}

// Setup checks the server and creates suite keys.
func Setup(label string) *Ctx {
	base := os.Getenv("BLOB_TEST_URL")
	if base == "" {
		base = "http://127.0.0.1:3099"
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if res, err := http.Get(base + "/health"); err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			log.Fatalf("server not reachable at %s", base)
		}
		time.Sleep(time.Second)
	}
	c := &Ctx{Base: base, label: label}
	var err error
	if c.Bearer, err = createKey(label+"-admin", "read,write,delete"); err != nil {
		log.Fatal(err)
	}
	if c.ReadKey, err = createKey(label+"-read", "read"); err != nil {
		log.Fatal(err)
	}
	if c.S3ID, c.S3Secret, err = createS3Key(label+"-s3", "read,write,delete"); err != nil {
		log.Fatal(err)
	}
	return c
}

// Cleanup revokes the suite keys.
func (c *Ctx) Cleanup() {
	for _, l := range []string{c.label + "-admin", c.label + "-read", c.label + "-s3"} {
		revokeByLabel(l)
	}
}

// Bucket returns an isolated bucket name for one test.
func (c *Ctx) Bucket(t *testing.T, op string) string {
	t.Helper()
	return fmt.Sprintf("e2e-%s-%s-%d", c.label, op, time.Now().UnixNano())
}

func runAdmin(args ...string) (string, error) {
	cmd := exec.Command("go", append([]string{"run", "."}, append([]string{"admin"}, args...)...)...)
	cmd.Dir = "../../.."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func createKey(label, scopes string) (string, error) {
	out, err := runAdmin("create", "--label", label, "--scopes", scopes)
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "token (shown once):") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "token (shown once):")), nil
		}
	}
	return "", fmt.Errorf("token not in output: %s", out)
}

func createS3Key(label, scopes string) (string, string, error) {
	out, err := runAdmin("create", "--label", label, "--scopes", scopes, "--s3")
	if err != nil {
		return "", "", fmt.Errorf("%v: %s", err, out)
	}
	var id, secret string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "access key (shown once):") {
			id = strings.TrimSpace(strings.TrimPrefix(line, "access key (shown once):"))
		}
		if strings.HasPrefix(line, "secret (shown once):") {
			secret = strings.TrimSpace(strings.TrimPrefix(line, "secret (shown once):"))
		}
	}
	if id == "" || secret == "" {
		return "", "", fmt.Errorf("creds not in output: %s", out)
	}
	return id, secret, nil
}

func revokeByLabel(label string) {
	out, err := runAdmin("list")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[1] == label {
			_, _ = runAdmin("revoke", f[0])
		}
	}
}

// Do sends req and returns status + body.
func Do(t *testing.T, req *http.Request) (int, []byte) {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, b
}

// BearerReq builds an authenticated request.
func BearerReq(t *testing.T, method, url, key, body, ctype string) *http.Request {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	return req
}

// --- minimal SigV4 signer (header + presigned), stdlib only ---

func hmacSHA256(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

// Sha256hex hashes bytes to hex.
func Sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// EmptySHA256 is the payload hash of an empty body.
const EmptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func escapeSigV4(s string) string {
	const hexd = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexd[c>>4])
		b.WriteByte(hexd[c&15])
	}
	return b.String()
}

func canonicalQuery(q url.Values) string {
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

func signingKey(secret, date, region, service string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), date)
	k = hmacSHA256(k, region)
	k = hmacSHA256(k, service)
	return hmacSHA256(k, "aws4_request")
}

// SignV4 signs req in place (header auth).
func SignV4(req *http.Request, id, secret, region, service string, t time.Time, payloadHash string) {
	amzDate := t.UTC().Format("20060102T150405Z")
	datestamp := t.UTC().Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	names := []string{}
	for name := range req.Header {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	var lines []string
	for _, n := range names {
		lines = append(lines, n+":"+strings.Join(strings.Fields(req.Header.Get(n)), " "))
	}
	canonical := strings.Join([]string{
		req.Method, req.URL.EscapedPath(), canonicalQuery(req.URL.Query()),
		strings.Join(lines, "\n") + "\n", strings.Join(names, ";"), payloadHash,
	}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	scope := datestamp + "/" + region + "/" + service + "/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	sig := hex.EncodeToString(hmacSHA256(signingKey(secret, datestamp, region, service), toSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+id+"/"+scope+", SignedHeaders="+strings.Join(names, ";")+", Signature="+sig)
}

// PresignV4 returns a presigned GET/HEAD URL valid for ttl.
func PresignV4(rawURL, id, secret, region, service string, t time.Time, ttl time.Duration) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	amzDate := t.UTC().Format("20060102T150405Z")
	datestamp := t.UTC().Format("20060102")
	q := u.Query()
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", id+"/"+datestamp+"/"+region+"/"+service+"/aws4_request")
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", strconv.Itoa(int(ttl.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")
	u.RawQuery = q.Encode()
	names := []string{"host"}
	lines := []string{"host:" + u.Host}
	canonical := strings.Join([]string{
		"GET", u.EscapedPath(), canonicalQuery(u.Query()),
		strings.Join(lines, "\n") + "\n", strings.Join(names, ";"), "UNSIGNED-PAYLOAD",
	}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	scope := datestamp + "/" + region + "/" + service + "/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	sig := hex.EncodeToString(hmacSHA256(signingKey(secret, datestamp, region, service), toSign))
	q.Set("X-Amz-Signature", sig)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
