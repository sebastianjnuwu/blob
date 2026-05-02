package multipart

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var bucketNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

func validateBucketName(bucket string) bool {
	if !bucketNamePattern.MatchString(bucket) {
		return false
	}

	return !strings.Contains(bucket, "..")
}

func ensurePathWithinBase(basePath, targetPath string) (string, error) {
	realBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}

	realTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(realBasePath, realTargetPath)
	if err != nil {
		return "", err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New("target path escapes base path")
	}

	return realTargetPath, nil
}