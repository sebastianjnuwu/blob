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
	token := fs.String("token", "", "import an existing secret instead of generating one")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	*label = strings.TrimSpace(*label)
	if *label == "" || len(*label) > 120 {
		fmt.Fprintln(os.Stderr, "admin create: --label is required (max 120 chars)")
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
	secret := *token
	if secret == "" {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			fmt.Fprintln(os.Stderr, "admin create:", err)
			return 1
		}
		secret = hex.EncodeToString(raw)
	} else if len(secret) < 32 {
		fmt.Fprintln(os.Stderr, "admin create: --token must be at least 32 characters")
		return 2
	}
	row := models.ApiKey{Label: *label, TokenHash: auth.HashToken(secret), Scopes: strings.Join(parts, ","), Active: true}
	if err := database.DB.Create(&row).Error; err != nil {
		fmt.Fprintln(os.Stderr, "admin create:", err)
		return 1
	}
	fmt.Println("id:    ", row.ID)
	fmt.Println("label: ", row.Label)
	fmt.Println("scopes:", row.Scopes)
	fmt.Println("token (shown once):", secret)
	return 0
}

func adminList() int {
	var rows []models.ApiKey
	if err := database.DB.Order("created_at desc").Find(&rows).Error; err != nil {
		fmt.Fprintln(os.Stderr, "admin list:", err)
		return 1
	}
	for _, k := range rows {
		fmt.Printf("%s  %-24s  %-16s  active=%t\n", k.ID, k.Label, k.Scopes, k.Active)
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
