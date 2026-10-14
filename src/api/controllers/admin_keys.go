package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"

	"github.com/google/uuid"
)

// knownScopes are the accepted permission names for new keys.
var knownScopes = map[string]bool{"read": true, "write": true, "delete": true, "all": true, "admin": true}

type adminKeyBody struct {
	Label  string `json:"label"`
	Scopes string `json:"scopes"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// AdminListKeys handles GET /admin/keys (admin). Hashes never leave the server.
func AdminListKeys(w http.ResponseWriter, r *http.Request) {
	var rows []models.ApiKey
	if err := database.DB.Order("created_at desc").Find(&rows).Error; err != nil {
		functions.WriteJSONError(w, "Failed to list keys", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": rows})
}

// AdminCreateKey handles POST /admin/keys (admin). The token is shown once.
func AdminCreateKey(w http.ResponseWriter, r *http.Request) {
	var body adminKeyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		functions.WriteJSONError(w, "Invalid body", http.StatusBadRequest)
		return
	}
	body.Label = strings.TrimSpace(body.Label)
	if body.Label == "" || len(body.Label) > 120 {
		functions.WriteJSONError(w, "Invalid label", http.StatusBadRequest)
		return
	}
	scopes := []string{}
	for _, p := range strings.Split(body.Scopes, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if !knownScopes[p] {
			functions.WriteJSONError(w, "Invalid scope: "+p, http.StatusBadRequest)
			return
		}
		scopes = append(scopes, p)
	}
	if len(scopes) == 0 {
		functions.WriteJSONError(w, "Invalid scopes", http.StatusBadRequest)
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		functions.WriteJSONError(w, "Failed to generate key", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))

	row := models.ApiKey{Label: body.Label, TokenHash: hex.EncodeToString(sum[:]), Scopes: strings.Join(scopes, ","), Active: true}
	if err := database.DB.Create(&row).Error; err != nil {
		functions.WriteJSONError(w, "Failed to create key", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": row.ID, "label": row.Label, "scopes": row.Scopes, "token": token})
}

// AdminRevokeKey handles DELETE /admin/keys/{id} (admin). Soft revoke.
func AdminRevokeKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strings.CutPrefix(r.URL.Path, "/admin/keys/")
	id = strings.Trim(id, "/")
	uid, err := uuid.Parse(id)
	if err != nil {
		functions.WriteJSONError(w, "Invalid key id", http.StatusBadRequest)
		return
	}
	res := database.DB.Model(&models.ApiKey{}).Where("id = ? AND active = ?", uid, true).Update("active", false)
	if res.Error != nil {
		functions.WriteJSONError(w, "Failed to revoke key", http.StatusInternalServerError)
		return
	}
	if res.RowsAffected == 0 {
		functions.WriteJSONError(w, "Key not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": id})
}
