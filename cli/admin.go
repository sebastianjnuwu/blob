package cli

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"blob/src/auth"
	"blob/src/database"
	"blob/src/models"

	"github.com/google/uuid"
)

// runAdmin implements `blob admin create|list|revoke`. API keys are created
// by an operator here, never auto-generated, never served over HTTP.
func RunAdmin(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: blob admin <create|list|revoke> [flags]")
		return 2
	}
	database.Postgres()
	if database.DB == nil {
		fmt.Fprintln(os.Stderr, "admin: database unavailable")
		return 1
	}
	switch args[0] {
	case "create":
		return adminCreate(args[1:])
	case "list":
		return adminList()
	case "revoke":
		return adminRevoke(args[1:])
	default:
		fmt.Fprintln(os.Stderr, "usage: blob admin <create|list|revoke> [flags]")
		return 2
	}
}

func adminCreate(args []string) int {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	label := fs.String("label", "", "key label (e.g. an admin email)")
	scopes := fs.String("scopes", "", "comma-separated: read,write,delete,all,admin")
	asS3 := fs.Bool("s3", false, "create S3-style credentials (access key + secret)")
	asBearer := fs.Bool("bearer", false, "create a bearer token (default)")
	token := fs.String("token", "", "import an existing bearer secret instead of generating one")
	accessKey := fs.String("access-key", "", "import an existing access key id instead of generating one (with --s3)")
	secret := fs.String("secret", "", "import an existing secret instead of generating one (with --s3)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	*label = strings.TrimSpace(*label)
	if *label == "" || len(*label) > 120 {
		fmt.Fprintln(os.Stderr, "admin create: --label is required (max 120 chars)")
		return 2
	}
	if *asS3 && *asBearer {
		fmt.Fprintln(os.Stderr, "admin create: --s3 and --bearer are exclusive")
		return 2
	}
	known := map[string]bool{"read": true, "write": true, "delete": true, "all": true, "admin": true}
	parts := []string{}
	for _, p := range strings.Split(*scopes, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !known[p] {
			fmt.Fprintln(os.Stderr, "admin create: invalid scope "+p)
			return 2
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		fmt.Fprintln(os.Stderr, "admin create: --scopes is required")
		return 2
	}
	row := models.ApiKey{Label: *label, Scopes: strings.Join(parts, ","), Active: true}
	if *asS3 {
		id := *accessKey
		if id == "" {
			var err error
			if id, err = randomString("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", 20); err != nil {
				fmt.Fprintln(os.Stderr, "admin create:", err)
				return 1
			}
		}
		key := *secret
		if key == "" {
			var err error
			if key, err = randomString("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", 40); err != nil {
				fmt.Fprintln(os.Stderr, "admin create:", err)
				return 1
			}
		} else if len(key) < 32 {
			fmt.Fprintln(os.Stderr, "admin create: --secret must be at least 32 characters")
			return 2
		}
		row.AccessKeyID = &id
		row.Secret = key
		if err := database.DB.Create(&row).Error; err != nil {
			fmt.Fprintln(os.Stderr, "admin create:", err)
			return 1
		}
		fmt.Println("id:        ", row.ID)
		fmt.Println("label:     ", row.Label)
		fmt.Println("scopes:    ", row.Scopes)
		fmt.Println("access key (shown once):", id)
		fmt.Println("secret (shown once):    ", key)
		return 0
	}
	if *token != "" && (*accessKey != "" || *secret != "") {
		fmt.Fprintln(os.Stderr, "admin create: --token is bearer-only (use --access-key/--secret with --s3)")
		return 2
	}
	raw := *token
	if raw == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			fmt.Fprintln(os.Stderr, "admin create:", err)
			return 1
		}
		raw = hex.EncodeToString(b)
	} else if len(raw) < 32 {
		fmt.Fprintln(os.Stderr, "admin create: --token must be at least 32 characters")
		return 2
	}
	h := auth.HashToken(raw)
	row.TokenHash = &h
	if err := database.DB.Create(&row).Error; err != nil {
		fmt.Fprintln(os.Stderr, "admin create:", err)
		return 1
	}
	fmt.Println("id:    ", row.ID)
	fmt.Println("label: ", row.Label)
	fmt.Println("scopes:", row.Scopes)
	fmt.Println("token (shown once):", raw)
	return 0
}

// randomString returns n random chars from alphabet.
func randomString(alphabet string, n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw), nil
}

func adminList() int {
	var rows []models.ApiKey
	if err := database.DB.Order("created_at desc").Find(&rows).Error; err != nil {
		fmt.Fprintln(os.Stderr, "admin list:", err)
		return 1
	}
	for _, k := range rows {
		akid := ""
		if k.AccessKeyID != nil {
			akid = *k.AccessKeyID
		}
		fmt.Printf("%s  %-24s  %-16s  %-20s  active=%t\n", k.ID, k.Label, k.Scopes, akid, k.Active)
	}
	return 0
}

func adminRevoke(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: blob admin revoke <id>")
		return 2
	}
	uid, err := uuid.Parse(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "admin revoke: invalid id")
		return 2
	}
	res := database.DB.Model(&models.ApiKey{}).Where("id = ? AND active = ?", uid, true).Update("active", false)
	if res.Error != nil {
		fmt.Fprintln(os.Stderr, "admin revoke:", res.Error)
		return 1
	}
	if res.RowsAffected == 0 {
		fmt.Fprintln(os.Stderr, "admin revoke: key not found")
		return 1
	}
	fmt.Println("revoked:", uid)
	return 0
}
