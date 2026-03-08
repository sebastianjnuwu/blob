package controllers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"blob/src/database"
	"blob/src/models"

	"github.com/google/uuid"
)

// DeleteBlobController handles DELETE /blob/{id}
func DeleteBlobController(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
		       if len(parts) < 3 || parts[2] == "" {
			       w.WriteHeader(http.StatusBadRequest)
			       if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing blob id"}); err != nil {
				       // Optionally log: fmt.Println("failed to encode delete error json:", err)
			       }
			       return
		       }
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
		       if err != nil {
			       w.WriteHeader(http.StatusBadRequest)
			       if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid blob id"}); err != nil {
				       // Optionally log: fmt.Println("failed to encode delete error json:", err)
			       }
			       return
		       }

	var blob struct {
		Path string
	}
	db := database.DB.Table("blobs").Select("path").Where("id = ?", id).First(&blob)
		       if db.Error != nil {
			       w.WriteHeader(http.StatusNotFound)
			       if err := json.NewEncoder(w).Encode(map[string]string{"error": "Blob not found"}); err != nil {
				       // Optionally log: fmt.Println("failed to encode delete error json:", err)
			       }
			       return
		       }

	// Remove file from disk
	if blob.Path != "" {
		storagePath := os.Getenv("BLOB_STORAGE_PATH")
		if storagePath == "" {
			storagePath = "storage/uploads"
		}
		filePath := storagePath + string(os.PathSeparator) + blob.Path
		_ = os.Remove(filePath)
	}

	// Remove from DB
	result := database.DB.Delete(&models.Blob{}, "id = ?", id)
	if result.RowsAffected == 0 {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Blob not found"}); err != nil {
			// Optionally log: fmt.Println("failed to encode delete error json:", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Blob deleted successfully"}); err != nil {
		// Optionally log: fmt.Println("failed to encode delete success json:", err)
	}
}
