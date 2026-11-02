package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemDriver stores blobs on the local filesystem under a base path.
// Layout: {base}/{bucket}/{key}. Buckets may contain slashes (subfolders).
type FilesystemDriver struct {
	base string
}

// NewFilesystemDriver returns a driver rooted at the given base directory.
func NewFilesystemDriver(base string) *FilesystemDriver {
	return &FilesystemDriver{base: base}
}

// ensureInside guards against path traversal, resolving target relative to base.
func ensureInside(base, target string) (string, error) {
	realBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	realTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(realBase, realTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("path escapes storage base")
	}
	return realTarget, nil
}

// validateRelativePathComponent ensures user-controlled path parts are safe.
// It allows nested relative paths (for bucket subfolders) but rejects traversal
// and absolute paths.
func validateRelativePathComponent(name, value string) error {
	clean := filepath.Clean(value)
	if value == "" || clean == "." || clean == ".." || filepath.IsAbs(value) {
		return fmt.Errorf("invalid %s", name)
	}
	parts := strings.Split(clean, string(os.PathSeparator))
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return fmt.Errorf("invalid %s", name)
		}
	}
	return nil
}

// resolve converts a bucket/key pair into a safe absolute path.
func (d *FilesystemDriver) resolve(bucket, key string) (string, error) {
	if err := validateRelativePathComponent("bucket", bucket); err != nil {
		return "", err
	}
	if err := validateRelativePathComponent("key", key); err != nil {
		return "", err
	}
	joined := filepath.Join(d.base, bucket, key)
	return ensureInside(d.base, joined)
}

// Name implements StorageDriver.
func (d *FilesystemDriver) Name() string { return "filesystem" }

// Put implements StorageDriver.
func (d *FilesystemDriver) Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return out.Sync()
}

// Create implements StorageDriver.
func (d *FilesystemDriver) Create(ctx context.Context, bucket, key string) (io.WriteCloser, error) {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}
	return os.Create(path)
}

// Open implements StorageDriver.
func (d *FilesystemDriver) Open(ctx context.Context, bucket, key string) (io.ReadSeekCloser, error) {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Stat implements StorageDriver.
func (d *FilesystemDriver) Stat(ctx context.Context, bucket, key string) (*FileInfo, error) {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{Size: info.Size(), ModTime: info.ModTime()}, nil
}

// Exists implements StorageDriver.
func (d *FilesystemDriver) Exists(ctx context.Context, bucket, key string) (bool, error) {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Delete implements StorageDriver.
func (d *FilesystemDriver) Delete(ctx context.Context, bucket, key string) error {
	path, err := d.resolve(bucket, key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	d.pruneEmptyParents(path)
	return nil
}

// pruneEmptyParents removes empty parent directories up to (but not including)
// the storage base, so deleting a file also cleans up empty bucket folders.
func (d *FilesystemDriver) pruneEmptyParents(path string) {
	realBase, err := filepath.Abs(d.base)
	if err != nil {
		return
	}
	realBase = filepath.Clean(realBase)

	realPath, err := filepath.Abs(path)
	if err != nil {
		return
	}
	dir := filepath.Clean(filepath.Dir(realPath))

	for dir != realBase {
		rel, err := filepath.Rel(realBase, dir)
		if err != nil {
			return
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if len(entries) > 0 {
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = parent
	}
}

// Move implements StorageDriver.
func (d *FilesystemDriver) Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src, err := d.resolve(srcBucket, srcKey)
	if err != nil {
		return err
	}
	dst, err := d.resolve(dstBucket, dstKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		d.pruneEmptyParents(src)
		return nil
	}
	// Fallback: copy + delete (works across filesystems)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	d.pruneEmptyParents(src)
	return nil
}

// EnsureBucket implements StorageDriver.
func (d *FilesystemDriver) EnsureBucket(ctx context.Context, bucket string) error {
	if err := validateRelativePathComponent("bucket", bucket); err != nil {
		return err
	}
	path, err := ensureInside(d.base, filepath.Join(d.base, bucket))
	if err != nil {
		return err
	}
	return os.MkdirAll(path, 0o750)
}

// RemoveTree implements StorageDriver.
func (d *FilesystemDriver) RemoveTree(ctx context.Context, bucket, keyPrefix string) error {
	path, err := d.resolve(bucket, keyPrefix)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(path)
}
