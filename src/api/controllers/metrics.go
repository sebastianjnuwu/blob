package controllers

import (
	"blob/src/database"
	"blob/src/lib/logger"
	"blob/src/metrics"
	"blob/src/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
)

func formatSize(size float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.2f %s", size, units[i])
}

func BlobMetricsController(w http.ResponseWriter, r *http.Request) {

	maxStorage := int64(0)
	if v := os.Getenv("BLOB_MAX_STORAGE_SIZE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			maxStorage = parsed
		}
	}

	var totalBlobs int64
	database.DB.Model(&models.Blob{}).Count(&totalBlobs)

	var totalSizeNull sql.NullInt64
	database.DB.Model(&models.Blob{}).Select("SUM(size)").Scan(&totalSizeNull)
	totalSize := int64(0)
	if totalSizeNull.Valid {
		totalSize = totalSizeNull.Int64
	}

	storageFree := maxStorage - totalSize

	var avgSizeNull sql.NullFloat64
	database.DB.Model(&models.Blob{}).Select("AVG(size)").Scan(&avgSizeNull)
	avgSize := float64(0)
	if avgSizeNull.Valid {
		avgSize = avgSizeNull.Float64
	}

	var downloadCountNull sql.NullInt64
	database.DB.Model(&models.Blob{}).Select("SUM(download_count)").Scan(&downloadCountNull)
	downloadCount := int64(0)
	if downloadCountNull.Valid {
		downloadCount = downloadCountNull.Int64
	}

	var viewCountNull sql.NullInt64
	database.DB.Model(&models.Blob{}).Select("SUM(view_count)").Scan(&viewCountNull)
	viewCount := int64(0)
	if viewCountNull.Valid {
		viewCount = viewCountNull.Int64
	}

	var maxSizeNull sql.NullInt64
	database.DB.Model(&models.Blob{}).Select("MAX(size)").Scan(&maxSizeNull)
	maxSize := int64(0)
	if maxSizeNull.Valid {
		maxSize = maxSizeNull.Int64
	}

	var minSizeNull sql.NullInt64
	database.DB.Model(&models.Blob{}).Select("MIN(size)").Scan(&minSizeNull)
	minSize := int64(0)
	if minSizeNull.Valid {
		minSize = minSizeNull.Int64
	}

	var publicCount int64
	database.DB.Model(&models.Blob{}).Where("public = true").Count(&publicCount)

	var privateCount int64
	database.DB.Model(&models.Blob{}).Where("public = false OR public IS NULL").Count(&privateCount)

	var expiredCount int64
	database.DB.Model(&models.Blob{}).Where("expires_at IS NOT NULL AND expires_at < NOW()").Count(&expiredCount)

	var multipartCompleted int64
	database.DB.Model(&models.MultipartUpload{}).Where("completed = true").Count(&multipartCompleted)

	// buckets
	type BucketStat struct {
		Bucket  string
		Count   int64
		Size    int64
		Public  int64
		Private int64
	}
	var bucketStats []BucketStat
	database.DB.Model(&models.Blob{}).
		Select("bucket, COUNT(*) as count, SUM(size) as size, SUM(CASE WHEN public = true THEN 1 ELSE 0 END) as public, SUM(CASE WHEN public = false OR public IS NULL THEN 1 ELSE 0 END) as private").
		Group("bucket").Scan(&bucketStats)

	buckets := make([]map[string]interface{}, 0, len(bucketStats))
	for _, b := range bucketStats {
		buckets = append(buckets, map[string]interface{}{
			"name":  b.Bucket,
			"blobs": b.Count,
			"size":  formatSize(float64(b.Size)),
			"visibility": map[string]interface{}{
				"public":  b.Public,
				"private": b.Private,
			},
		})
	}

	// tipos MIME
	type TypeStat struct {
		Mime  string
		Count int64
		Size  int64
	}
	var typeStats []TypeStat
	database.DB.Model(&models.Blob{}).
		Select("mime, COUNT(*) as count, SUM(size) as size").
		Group("mime").Scan(&typeStats)

	types := make([]map[string]interface{}, 0, len(typeStats))
	for _, t := range typeStats {
		types = append(types, map[string]interface{}{
			"mime":  t.Mime,
			"count": t.Count,
			"size":  formatSize(float64(t.Size)),
		})
	}

	// último upload
	var lastUpload models.Blob
	database.DB.Model(&models.Blob{}).Order("created_at desc").First(&lastUpload)

	summary := map[string]interface{}{
		"total_blobs":         totalBlobs,
		"total_size":          formatSize(float64(totalSize)),
		"average_size":        formatSize(avgSize),
		"max_size":            formatSize(float64(maxSize)),
		"min_size":            formatSize(float64(minSize)),
		"multipart_completed": multipartCompleted,
		"total_downloads":     downloadCount,
		"total_views":         viewCount,
		"storage_max":         formatSize(float64(maxStorage)),
		"storage_free":        formatSize(float64(storageFree)),
	}

	// Runtime (in-process) telemetry
	runtime := metrics.Default.Snapshot()
	summary["runtime_uploads"] = runtime.UploadsTotal
	summary["runtime_downloads"] = runtime.DownloadsTotal
	summary["runtime_views"] = runtime.ViewsTotal
	summary["runtime_errors"] = runtime.ErrorsTotal
	summary["uploads_per_minute"] = runtime.UploadsPerMinute
	summary["downloads_per_minute"] = runtime.DownloadsPerMin
	summary["avg_upload_ms"] = runtime.AvgUploadMs
	summary["avg_download_ms"] = runtime.AvgDownloadMs

	lastUploadMap := map[string]interface{}{
		"bucket":     lastUpload.Bucket,
		"filename":   lastUpload.Filename,
		"id":         lastUpload.ID,
		"created_at": lastUpload.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"summary":     summary,
		"last_upload": lastUploadMap,
		"buckets":     buckets,
		"types":       types,
	}); err != nil {
		logger.Error("failed to encode metrics json: %v", err)
	}
}
