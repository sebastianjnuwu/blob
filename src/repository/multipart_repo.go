package repository

import (
	"context"
	"encoding/json"
	"time"

	"blob/src/models"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MultipartRepository abstracts persistence of multipart upload sessions.
type MultipartRepository struct {
	db *gorm.DB
}

func NewMultipartRepository(db *gorm.DB) *MultipartRepository {
	return &MultipartRepository{db: db}
}

func (r *MultipartRepository) Create(ctx context.Context, upload *models.MultipartUpload) error {
	return r.db.WithContext(ctx).Create(upload).Error
}

func (r *MultipartRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MultipartUpload, error) {
	var upload models.MultipartUpload
	if err := r.db.WithContext(ctx).First(&upload, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &upload, nil
}

func (r *MultipartRepository) UpdateChunks(ctx context.Context, id uuid.UUID, chunks []int) error {
	raw, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&models.MultipartUpload{}).
		Where("id = ?", id).
		Update("chunks_done", datatypes.JSON(raw)).Error
}

func (r *MultipartRepository) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.MultipartUpload{}).
		Where("id = ?", id).
		Update("completed", true).Error
}

// FindIncomplete returns unfinished upload sessions, newest last.
func (r *MultipartRepository) FindIncomplete(ctx context.Context, limit int) ([]models.MultipartUpload, error) {
	var uploads []models.MultipartUpload
	err := r.db.WithContext(ctx).
		Where("(completed = ? OR completed IS NULL)", false).
		Order("created_at ASC").Limit(limit).Find(&uploads).Error
	return uploads, err
}

// FindIncompleteOlderThan returns unfinished upload sessions older than cutoff.
func (r *MultipartRepository) FindIncompleteOlderThan(ctx context.Context, cutoff time.Time) ([]models.MultipartUpload, error) {
	var uploads []models.MultipartUpload
	err := r.db.WithContext(ctx).
		Where("(completed = ? OR completed IS NULL) AND created_at < ?", false, cutoff).
		Find(&uploads).Error
	return uploads, err
}

func (r *MultipartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.MultipartUpload{}, "id = ?", id).Error
}
