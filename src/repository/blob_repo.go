package repository

import (
	"context"
	"strings"
	"time"

	"blob/src/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BlobRepository abstracts persistence of blob metadata.
type BlobRepository struct {
	db *gorm.DB
}

func NewBlobRepository(db *gorm.DB) *BlobRepository {
	return &BlobRepository{db: db}
}

func (r *BlobRepository) Create(ctx context.Context, blob *models.Blob) error {
	return r.db.WithContext(ctx).Create(blob).Error
}

func (r *BlobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Blob, error) {
	var blob models.Blob
	if err := r.db.WithContext(ctx).First(&blob, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &blob, nil
}

// GetByHash returns the most recent blob with the given content hash.
func (r *BlobRepository) GetByHash(ctx context.Context, hash string) (*models.Blob, error) {
	var blob models.Blob
	if err := r.db.WithContext(ctx).Where("hash = ?", hash).Order("created_at DESC").First(&blob).Error; err != nil {
		return nil, err
	}
	return &blob, nil
}

// ListFilter describes the filters for List.
type ListFilter struct {
	Bucket   string
	Search   string
	Prefix   string
	OrderKey bool
	Page     int
	PageSize int
}

// GetByBucketFilename returns the most recent live blob for an exact bucket+filename.
func (r *BlobRepository) GetByBucketFilename(ctx context.Context, bucket, filename string) (*models.Blob, error) {
	var blob models.Blob
	if err := r.db.WithContext(ctx).Where("bucket = ? AND filename = ? AND deleted_at IS NULL", bucket, filename).Order("created_at DESC").First(&blob).Error; err != nil {
		return nil, err
	}
	return &blob, nil
}

// Bucket holds a bucket name with its creation time (oldest object).
type Bucket struct {
	Name      string
	CreatedAt time.Time
}

// Buckets lists distinct buckets with live objects, oldest first.
func (r *BlobRepository) Buckets(ctx context.Context) ([]Bucket, error) {
	var out []Bucket
	if err := r.db.WithContext(ctx).Model(&models.Blob{}).Select("bucket AS name, MIN(created_at) AS created_at").Where("deleted_at IS NULL").Group("bucket").Order("name").Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// List returns a page of blobs and the total number of matches.
func (r *BlobRepository) List(ctx context.Context, f ListFilter) ([]models.Blob, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&models.Blob{}).Where("deleted_at IS NULL")
	if f.Bucket != "" {
		query = query.Where("bucket = ?", f.Bucket)
	}
	if f.Search != "" {
		query = query.Where("LOWER(filename) LIKE ?", "%"+strings.ToLower(f.Search)+"%")
	}
	if f.Prefix != "" {
		query = query.Where("filename LIKE ?", f.Prefix+"%")
	}
	order := "created_at DESC"
	if f.OrderKey {
		order = "filename ASC"
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blobs []models.Blob
	if err := query.Order(order).Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&blobs).Error; err != nil {
		return nil, 0, err
	}
	return blobs, total, nil
}

func (r *BlobRepository) Update(ctx context.Context, blob *models.Blob) error {
	return r.db.WithContext(ctx).Save(blob).Error
}

// Delete removes the metadata row.
func (r *BlobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Blob{}, "id = ?", id).Error
}

// FindByPath returns blobs that share the same physical path (used for
// refcount-aware deletion after deduplication).
func (r *BlobRepository) FindByPath(ctx context.Context, path string) ([]models.Blob, error) {
	var blobs []models.Blob
	err := r.db.WithContext(ctx).Where("path = ?", path).Find(&blobs).Error
	return blobs, err
}

func (r *BlobRepository) IncrementCounter(ctx context.Context, id uuid.UUID, field string) error {
	return r.db.WithContext(ctx).Model(&models.Blob{}).
		Where("id = ?", id).
		Update(field, gorm.Expr("COALESCE("+field+", 0) + 1")).Error
}

// FindExpired returns blobs whose expiry date has passed.
func (r *BlobRepository) FindExpired(ctx context.Context, before time.Time) ([]models.Blob, error) {
	var blobs []models.Blob
	err := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", before).
		Find(&blobs).Error
	return blobs, err
}
