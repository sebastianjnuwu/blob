package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
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

// MultipartService manages multipart upload sessions: creating them, storing
// chunks, verifying integrity, assembling the final object and cleaning up
// abandoned sessions.
type MultipartService struct {
	repo    *repository.MultipartRepository
	blobs   *BlobService
	storage storage.StorageDriver
	bus     *events.Bus
	metrics *metrics.Service
	cfg     *config.Config
}

// Multipart is the process-wide multipart service.
var Multipart *MultipartService

// InitMultipartService builds the default multipart service.
func InitMultipartService() *MultipartService {
	if Blobs == nil {
		InitBlobService()
	}
	Multipart = &MultipartService{
		repo:    repository.NewMultipartRepository(database.DB),
		blobs:   Blobs,
		storage: Blobs.storage,
		bus:     events.Default,
		metrics: metrics.Default,
		cfg:     Blobs.cfg,
	}
	return Multipart
}

// chunkBucket is the pseudo-bucket holding in-progress chunks.
const chunkBucket = "tmp"

// maxChunkStatusEntries bounds the chunk status response, preventing a single
// request from allocating a huge missing-chunks list (CPU/memory DoS).
const maxChunkStatusEntries = 10000

// chunkKey returns the storage key for a chunk of a session.
func chunkKey(uploadID uuid.UUID, index int) string {
	return fmt.Sprintf("%s/chunk_%d", uploadID.String(), index)
}

// InitiateInput carries validated fields for a new upload session.
type InitiateInput struct {
	UserID   uuid.UUID
	Bucket   string
	Filename string
	Size     int64
}

// Initiate creates an upload session and its temporary directory.
func (s *MultipartService) Initiate(ctx context.Context, in InitiateInput) (*models.MultipartUpload, error) {
	upload := models.MultipartUpload{
		UserID:     in.UserID,
		Bucket:     in.Bucket,
		Filename:   in.Filename,
		Size:       in.Size,
		ChunksDone: []byte("[]"),
		Completed:  false,
	}
	if err := s.repo.Create(ctx, &upload); err != nil {
		return nil, fmt.Errorf("failed to create upload session: %w", err)
	}
	if err := s.storage.EnsureBucket(ctx, chunkBucket); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	return &upload, nil
}

// ChunkInput carries a single chunk upload request.
type ChunkInput struct {
	UploadID uuid.UUID
	UserID   uuid.UUID
	Index    int
	Hash     string
	MaxSize  int64
	Body     io.Reader
}

// ErrUnauthorized is returned when a user tries to touch another user's session.
var ErrUnauthorized = fmt.Errorf("forbidden: not your upload")

// UploadChunk stores and verifies a single chunk.
func (s *MultipartService) UploadChunk(ctx context.Context, in ChunkInput) error {
	upload, err := s.repo.GetByID(ctx, in.UploadID)
	if err != nil {
		return fmt.Errorf("invalid uploadId")
	}
	if upload.UserID != in.UserID {
		return ErrUnauthorized
	}
	if upload.Completed {
		return fmt.Errorf("upload already completed")
	}
	if in.Hash == "" {
		return fmt.Errorf("missing X-Chunk-Hash header")
	}

	key := chunkKey(in.UploadID, in.Index)
	hasher := sha256.New()
	tee := io.TeeReader(in.Body, hasher)
	if err := s.storage.Put(ctx, chunkBucket, key, tee, in.MaxSize); err != nil {
		return fmt.Errorf("failed to save chunk")
	}

	written := -1
	// io.TeeReader does not expose the written count; stat the object instead.
	if info, err := s.storage.Stat(ctx, chunkBucket, key); err == nil {
		written = int(info.Size)
	}

	if s.cfg.MinChunkSize > 0 && written < int(s.cfg.MinChunkSize) {
		_ = s.storage.Delete(ctx, chunkBucket, key)
		return fmt.Errorf("chunk too small (min %d bytes)", s.cfg.MinChunkSize)
	}
	if s.cfg.MaxChunkSize > 0 && int64(written) > s.cfg.MaxChunkSize {
		_ = s.storage.Delete(ctx, chunkBucket, key)
		return fmt.Errorf("chunk too large (max %d bytes)", s.cfg.MaxChunkSize)
	}

	calculatedHash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(calculatedHash, in.Hash) {
		_ = s.storage.Delete(ctx, chunkBucket, key)
		return fmt.Errorf("chunk hash mismatch (integrity check failed)")
	}

	// Enforce the per-upload size cap against the actual bytes stored so far,
	// regardless of the declared size at initiate time.
	if s.cfg.MaxUploadSize > 0 {
		total := int64(written)
		var existing []int
		_ = json.Unmarshal(upload.ChunksDone, &existing)
		for _, idx := range existing {
			if idx == in.Index {
				continue
			}
			if info, err := s.storage.Stat(ctx, chunkBucket, chunkKey(in.UploadID, idx)); err == nil {
				total += info.Size
			}
		}
		if total > s.cfg.MaxUploadSize {
			_ = s.storage.Delete(ctx, chunkBucket, key)
			return fmt.Errorf("upload exceeds max size (%d bytes)", s.cfg.MaxUploadSize)
		}
	}

	var chunks []int
	_ = json.Unmarshal(upload.ChunksDone, &chunks)
	found := false
	for _, c := range chunks {
		if c == in.Index {
			found = true
			break
		}
	}
	if !found {
		chunks = append(chunks, in.Index)
		if err := s.repo.UpdateChunks(ctx, in.UploadID, chunks); err != nil {
			return err
		}
	}
	return nil
}

// UploadStatus describes the progress of an upload session.
type UploadStatus struct {
	Upload        *models.MultipartUpload `json:"upload"`
	ChunkSize     int                     `json:"chunk_size"`
	TotalChunks   int                     `json:"total_chunks"`
	MissingChunks []int                   `json:"missing_chunks"`
}

// Status reports which chunks are still missing for a session.
func (s *MultipartService) Status(ctx context.Context, uploadID, userID uuid.UUID, chunkSize int) (*UploadStatus, error) {
	upload, err := s.repo.GetByID(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("invalid uploadId")
	}
	if upload.UserID != userID {
		return nil, ErrUnauthorized
	}
	if chunkSize <= 0 {
		chunkSize = 102400
	}

	var chunksDone []int
	_ = json.Unmarshal(upload.ChunksDone, &chunksDone)
	chunkMap := make(map[int]bool)
	for _, idx := range chunksDone {
		chunkMap[idx] = true
	}
	totalChunks := int((upload.Size + int64(chunkSize) - 1) / int64(chunkSize))
	if totalChunks > maxChunkStatusEntries {
		return nil, fmt.Errorf("upload too large to report chunk status")
	}
	var missing []int
	for i := 0; i < totalChunks; i++ {
		if !chunkMap[i] {
			missing = append(missing, i)
		}
	}
	return &UploadStatus{
		Upload:        upload,
		ChunkSize:     chunkSize,
		TotalChunks:   totalChunks,
		MissingChunks: missing,
	}, nil
}

// CompleteInput carries the completion request.
type CompleteInput struct {
	UploadID      uuid.UUID
	UserID        uuid.UUID
	FinalHash     string
	SkipHashCheck bool
}

// Complete assembles the chunks, validates the final hash and promotes the
// session into a regular blob.
func (s *MultipartService) Complete(ctx context.Context, in CompleteInput) (*models.Blob, error) {
	if in.FinalHash == "" && !in.SkipHashCheck {
		return nil, fmt.Errorf("missing X-Final-Hash header")
	}
	upload, err := s.repo.GetByID(ctx, in.UploadID)
	if err != nil {
		return nil, fmt.Errorf("invalid uploadId")
	}
	if upload.UserID != in.UserID {
		return nil, ErrUnauthorized
	}
	if upload.Completed {
		return nil, fmt.Errorf("upload already completed")
	}

	var chunks []int
	if err := json.Unmarshal(upload.ChunksDone, &chunks); err != nil || len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks uploaded")
	}
	sort.Ints(chunks)

	// Assemble final file in the temp namespace.
	finalKey := in.UploadID.String() + "/final"
	final, err := s.storage.Create(ctx, chunkBucket, finalKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create final file")
	}

	closeWithError := func() {
		final.Close()
		_ = s.storage.Delete(ctx, chunkBucket, finalKey)
	}

	for _, idx := range chunks {
		chunk, err := s.storage.Open(ctx, chunkBucket, chunkKey(in.UploadID, idx))
		if err != nil {
			closeWithError()
			return nil, fmt.Errorf("missing chunk")
		}
		if _, err := io.Copy(final, chunk); err != nil {
			chunk.Close()
			closeWithError()
			return nil, fmt.Errorf("failed to write chunk")
		}
		chunk.Close()
	}
	if err := final.Close(); err != nil {
		_ = s.storage.Delete(ctx, chunkBucket, finalKey)
		return nil, fmt.Errorf("failed to close final file")
	}

	// Validate hash and detect MIME from the assembled file.
	mime, hash, size, err := inspectFile(ctx, s.storage, chunkBucket, finalKey)
	if err != nil {
		_ = s.storage.Delete(ctx, chunkBucket, finalKey)
		return nil, err
	}
	if !in.SkipHashCheck && !strings.EqualFold(hash, in.FinalHash) {
		_ = s.storage.Delete(ctx, chunkBucket, finalKey)
		return nil, fmt.Errorf("final file hash mismatch (integrity check failed)")
	}

	// Promote to the final bucket/key.
	blobID := in.UploadID
	if err := s.storage.Move(ctx, chunkBucket, finalKey, upload.Bucket, blobID.String()); err != nil {
		_ = s.storage.Delete(ctx, chunkBucket, finalKey)
		return nil, fmt.Errorf("failed to move final file")
	}

	bl := models.Blob{
		ID:        blobID,
		Bucket:    upload.Bucket,
		Filename:  upload.Filename,
		Mime:      mime,
		Size:      size,
		Hash:      hash,
		Path:      upload.Bucket + "/" + blobID.String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := Blobs.repo.Create(ctx, &bl); err != nil {
		return nil, fmt.Errorf("failed to save blob")
	}

	if err := s.repo.MarkCompleted(ctx, in.UploadID); err != nil {
		return nil, err
	}
	_ = s.storage.RemoveTree(ctx, chunkBucket, in.UploadID.String())

	s.bus.Publish(events.Event{
		Type:     events.MultipartCompleted,
		BlobID:   bl.ID,
		Bucket:   bl.Bucket,
		Filename: bl.Filename,
		Size:     bl.Size,
		Data:     map[string]interface{}{"hash": bl.Hash},
	})
	s.metrics.RecordUpload(bl.Size, 0)

	return &bl, nil
}

// inspectFile reads the first 512 bytes for MIME detection and hashes the whole
// stream, then seeks back to start.
func inspectFile(ctx context.Context, drv storage.StorageDriver, bucket, key string) (string, string, int64, error) {
	reader, err := drv.Open(ctx, bucket, key)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to open assembled file")
	}
	defer reader.Close()

	buf := make([]byte, 512)
	n, _ := reader.Read(buf)
	mime := http.DetectContentType(buf[:n])
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return "", "", 0, fmt.Errorf("failed to seek assembled file")
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, reader)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to hash assembled file")
	}
	return mime, hex.EncodeToString(hasher.Sum(nil)), size, nil
}

// Abort cancels a session and removes its temp chunks.
func (s *MultipartService) Abort(ctx context.Context, uploadID, userID uuid.UUID) error {
	upload, err := s.repo.GetByID(ctx, uploadID)
	if err != nil {
		return fmt.Errorf("invalid uploadId")
	}
	if upload.UserID != userID {
		return ErrUnauthorized
	}
	if upload.Completed {
		return fmt.Errorf("upload already completed")
	}
	_ = s.storage.RemoveTree(ctx, chunkBucket, uploadID.String())
	return s.repo.Delete(ctx, uploadID)
}

// PartInfo describes one uploaded part.
type PartInfo struct {
	Number int
	Size   int64
}

// Parts lists uploaded parts with sizes for an S3 session.
func (s *MultipartService) Parts(ctx context.Context, uploadID, userID uuid.UUID) ([]PartInfo, error) {
	upload, err := s.repo.GetByID(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("invalid uploadId")
	}
	if upload.UserID != userID {
		return nil, ErrUnauthorized
	}
	if upload.Completed {
		return nil, fmt.Errorf("upload already completed")
	}
	var indices []int
	_ = json.Unmarshal(upload.ChunksDone, &indices)
	sort.Ints(indices)
	out := make([]PartInfo, 0, len(indices))
	for _, idx := range indices {
		size := int64(0)
		if info, err := s.storage.Stat(ctx, chunkBucket, chunkKey(uploadID, idx)); err == nil {
			size = info.Size
		}
		out = append(out, PartInfo{Number: idx, Size: size})
	}
	return out, nil
}

// FindIncomplete lists unfinished upload sessions.
func (s *MultipartService) FindIncomplete(ctx context.Context, limit int) ([]models.MultipartUpload, error) {
	return s.repo.FindIncomplete(ctx, limit)
}

// CleanupAbandoned removes sessions and temp data older than the cutoff.
// Returns the number of sessions removed.
func (s *MultipartService) CleanupAbandoned(ctx context.Context, cutoff time.Time) (int, error) {
	uploads, err := s.repo.FindIncompleteOlderThan(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, upload := range uploads {
		_ = s.storage.RemoveTree(ctx, chunkBucket, upload.ID.String())
		if !upload.Completed {
			if err := s.repo.Delete(ctx, upload.ID); err != nil {
				continue
			}
		}
		removed++
	}
	return removed, nil
}
