package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"blob/src/database"
	"blob/src/models"
)

// ListBlobsController handles GET /blob
func ListBlobsController(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	bucket := q.Get("bucket")
	search := q.Get("search")
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}

	db := database.DB.Model(&models.Blob{})
	if bucket != "" {
		db = db.Where("bucket = ?", bucket)
	}
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		db = db.Where("LOWER(filename) LIKE ?", like)
	}

	var total int64
	db.Count(&total)

	var blobs []models.Blob
	db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&blobs)

	w.Header().Set("Content-Type", "application/json")
	pages := 1
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"meta": map[string]interface{}{
			"page":     page,
			"per_page": pageSize,
			"count":    len(blobs),
			"pages":    pages,
			"total":    total,
		},
		"blobs": blobs,
	}); err != nil {
		// Optionally log: fmt.Println("failed to encode list json:", err)
	}
}
