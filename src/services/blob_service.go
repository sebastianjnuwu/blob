package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"blob/src/config"
	"blob/src/database"
	"blob/src/events"
	"blob/src/metrics"
	"blob/src/models"
	"blob/src/repository"
	"blob/src/storage"

	"github.com/google/uuid"
)

// BlobService orchestrates blob operations, separating the HTTP layer from the
// domain logic. It depends on a storage driver and metadata repository so that
// neither the transport nor the persistence details leak into the handlers.
type BlobService struct {
	repo    *repository.BlobRepository
	storage storage.StorageDriver
	bus     *events.Bus
	metrics *metrics.Service
	cfg     *config.Config
}

// Blobs is the process-wide blob service.
var Blobs *BlobService

// InitBlobService builds the default blob service.
func InitBlobService() *BlobService {
	cfg := config.Env
	if cfg == nil {
		cfg = config.Load()
	}
	driver, err := storage.New("", cfg)
	if err != nil {
		// filesystem driver never fails to construct
		panic(err)
	}
	Blobs = &BlobService{
		repo:    repository.NewBlobRepository(database.DB),
		storage: driver,
		bus:     events.Default,
		metrics: metrics.Default,
		cfg:     cfg,
	}
	return Blobs
}

// BucketKey splits a stored path ("bucket/key") into its bucket and key parts.
// Buckets may contain slashes (subfolders), so only the first segment is the
// bucket and the remainder is the key.
func BucketKey(path string) (bucket, key string) {
	idx := strings.IndexByte(path, '/')
	if idx < 0 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}

// countingReader wraps a reader and tracks the number of bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// UploadInput carries validated fields from the HTTP layer.
type UploadInput struct {
	Bucket    string
	Filename  string
	Mime      string
	Size      int64
	Public    bool
	ExpiresAt *time.Time
	Metadata  map[string]interface{}
	File      io.Reader
}

// Upload streams the file into storage, computes its SHA256, applies
// deduplication when enabled and persists the metadata row.
func (s *BlobService) Upload(ctx context.Context, in UploadInput) (*models.Blob, error) {
	id := uuid.New()
	bucket := in.Bucket
	key := id.String()

	if err := s.storage.EnsureBucket(ctx, bucket); err != nil {
		return nil, fmt.Errorf("failed to create bucket directory: %w", err)
	}

	hasher := sha256.New()
	counting := &countingReader{r: in.File}
	tee := io.TeeReader(counting, hasher)

	if err := s.storage.Put(ctx, bucket, key, tee, in.Size); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	path := bucket + "/" + key
	size := counting.n

	// Deduplication: if the same content already exists and physical
	// storage is still available, reuse it instead of keeping a copy.
	if s.cfg.DedupEnabled {
		if existing, err := s.repo.GetByHash(ctx, hash); err == nil {
			exBucket, exKey := BucketKey(existing.Path)
			ok, _ := s.storage.Exists(ctx, exBucket, exKey)
			if ok {
				_ = s.storage.Delete(ctx, bucket, key)
				path = existing.Path
				size = existing.Size
			}
		}
	}

	blob := models.Blob{
		ID:        id,
		Bucket:    bucket,
		Filename:  in.Filename,
		Mime:      in.Mime,
		Size:      size,
		Hash:      hash,
		Path:      path,
		Public:    &in.Public,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: in.ExpiresAt,
	}
	if in.Metadata != nil {
		raw, _ := json.Marshal(in.Metadata)
		blob.Metadata = raw
	}

	if err := s.repo.Create(ctx, &blob); err != nil {
		return nil, fmt.Errorf("failed to save blob metadata: %w", err)
	}

	s.bus.Publish(events.Event{
		Type:     events.BlobUploaded,
		BlobID:   blob.ID,
		Bucket:   blob.Bucket,
		Filename: blob.Filename,
		Size:     blob.Size,
		Data:     map[string]interface{}{"hash": blob.Hash, "path": blob.Path},
	})

	return &blob, nil
}

// ListResult wraps a page of blobs with pagination metadata.
type ListResult struct {
	Blobs    []models.Blob `json:"blobs"`
	Page     int           `json:"page"`
	PageSize int           `json:"per_page"`
	Count    int           `json:"count"`
	Pages    int           `json:"pages"`
	Total    int64         `json:"total"`
}

// GetByBucketFilename returns the most recent live blob for an exact bucket+filename.
func (s *BlobService) GetByBucketFilename(ctx context.Context, bucket, filename string) (*models.Blob, error) {
	return s.repo.GetByBucketFilename(ctx, bucket, filename)
}

// Buckets lists distinct buckets with live objects.
func (s *BlobService) Buckets(ctx context.Context) ([]repository.Bucket, error) {
	return s.repo.Buckets(ctx)
}

// Clone persists a pre-built row (used by S3 CopyObject; shares the file).
func (s *BlobService) Clone(ctx context.Context, blob *models.Blob) error {
	if err := s.repo.Create(ctx, blob); err != nil {
		return err
	}
	s.bus.Publish(events.Event{
		Type:     events.BlobUploaded,
		BlobID:   blob.ID,
		Bucket:   blob.Bucket,
		Filename: blob.Filename,
		Size:     blob.Size,
		Data:     map[string]interface{}{"hash": blob.Hash, "path": blob.Path, "copy": true},
	})
	return nil
}

// List returns a paginated, filtered list of blobs.
func (s *BlobService) List(ctx context.Context, f repository.ListFilter) (*ListResult, error) {
	blobs, total, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	pages := 1
	if total > 0 {
		pages = int((total + int64(f.PageSize) - 1) / int64(f.PageSize))
	}
	return &ListResult{
		Blobs:    blobs,
		Page:     f.Page,
		PageSize: f.PageSize,
		Count:    len(blobs),
		Pages:    pages,
		Total:    total,
	}, nil
}

// Get fetches a blob by ID.
func (s *BlobService) Get(ctx context.Context, id uuid.UUID) (*models.Blob, error) {
	return s.repo.GetByID(ctx, id)
}

// EditInput carries optional updates to a blob.
type EditInput struct {
	Metadata  map[string]interface{}
	Public    *bool
	ExpiresAt *time.Time
	Bucket    *string
	Filename  *string
}

// Edit applies field updates and moves the physical file when the bucket changes.
func (s *BlobService) Edit(ctx context.Context, id uuid.UUID, in EditInput) (*models.Blob, error) {
	blob, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Metadata != nil {
		raw, _ := json.Marshal(in.Metadata)
		blob.Metadata = raw
	}
	if in.Public != nil {
		blob.Public = in.Public
	}
	if in.ExpiresAt != nil {
		blob.ExpiresAt = in.ExpiresAt
	}
	if in.Filename != nil && *in.Filename != "" {
		blob.Filename = *in.Filename
	}
	if in.Bucket != nil && *in.Bucket != "" {
		oldBucket := blob.Bucket
		blob.Bucket = *in.Bucket
		blob.Path = blob.Bucket + "/" + blob.ID.String()
		srcBucket, srcKey := BucketKey(oldBucket + "/" + blob.ID.String())
		dstBucket, dstKey := BucketKey(blob.Path)
		if err := s.storage.Move(ctx, srcBucket, srcKey, dstBucket, dstKey); err != nil {
			blob.Bucket = oldBucket
			blob.Path = oldBucket + "/" + blob.ID.String()
			return nil, fmt.Errorf("failed to move file: %w", err)
		}
	}

	blob.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, blob); err != nil {
		return nil, err
	}
	return blob, nil
}

// Delete removes the metadata row and, when no other blob shares the physical
// file (deduplication), the file itself.
func (s *BlobService) Delete(ctx context.Context, id uuid.UUID) error {
	blob, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.removePhysicalIfUnshared(ctx, blob.Path); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.bus.Publish(events.Event{
		Type:     events.BlobDeleted,
		BlobID:   blob.ID,
		Bucket:   blob.Bucket,
		Filename: blob.Filename,
		Size:     blob.Size,
	})
	return nil
}

// removePhysicalIfUnshared deletes the physical object unless another metadata
// row still references the same path (shared by deduplication).
func (s *BlobService) removePhysicalIfUnshared(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	sharers, err := s.repo.FindByPath(ctx, path)
	if err != nil {
		return err
	}
	if len(sharers) > 1 {
		return nil
	}
	bucket, key := BucketKey(path)
	return s.storage.Delete(ctx, bucket, key)
}

// BlobFile is an open, seekable file ready to be streamed.
type BlobFile struct {
	Blob    *models.Blob
	Reader  io.ReadSeekCloser
	Size    int64
	ModTime time.Time
}

// OpenFile opens the physical file for a blob and increments a counter
// (download_count or view_count). The caller is responsible for closing.
func (s *BlobService) OpenFile(ctx context.Context, id uuid.UUID, counterField string) (*BlobFile, error) {
	blob, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	bucket, key := BucketKey(blob.Path)
	info, err := s.storage.Stat(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	reader, err := s.storage.Open(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if counterField != "" {
		_ = s.repo.IncrementCounter(ctx, id, counterField)
	}
	return &BlobFile{Blob: blob, Reader: reader, Size: info.Size, ModTime: info.ModTime}, nil
}

// PurgeExpired removes all blobs whose expiration has passed. It is used by
// the expiration worker.
func (s *BlobService) PurgeExpired(ctx context.Context) (int, error) {
	expired, err := s.repo.FindExpired(ctx, time.Now())
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, blob := range expired {
		if err := s.removePhysicalIfUnshared(ctx, blob.Path); err != nil {
			continue
		}
		if err := s.repo.Delete(ctx, blob.ID); err != nil {
			continue
		}
		s.bus.Publish(events.Event{
			Type:     events.BlobExpired,
			BlobID:   blob.ID,
			Bucket:   blob.Bucket,
			Filename: blob.Filename,
			Size:     blob.Size,
		})
		removed++
	}
	return removed, nil
}
