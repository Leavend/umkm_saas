package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStore persists assets onto the local filesystem. It is intended for
// development and test environments where an object storage service is not
// available.
type FileStore struct {
	basePath string
}

// NewFileStore initializes a FileStore rooted at basePath.
func NewFileStore(basePath string) (*FileStore, error) {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return nil, errors.New("storage: base path is required")
	}
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("storage: ensure base path: %w", err)
	}
	return &FileStore{basePath: basePath}, nil
}

// BasePath returns the configured root directory.
func (s *FileStore) BasePath() string {
	if s == nil {
		return ""
	}
	return s.basePath
}

// Write persists the provided bytes at the given relative key and returns the
// canonicalized storage key. Keys are cleaned to prevent directory traversal.
func (s *FileStore) Write(ctx context.Context, key string, data []byte) (string, error) {
	if s == nil {
		return "", errors.New("storage: no store configured")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cleanKey, err := sanitizeKey(key)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(s.basePath, filepath.FromSlash(cleanKey))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", fmt.Errorf("storage: ensure directory: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", fmt.Errorf("storage: write file: %w", err)
	}
	return cleanKey, nil
}

// sanitizeKey normalizes a key and prevents escaping the storage root.
func sanitizeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("storage: key is required")
	}
	key = strings.ReplaceAll(key, "\\", "/")
	key = strings.TrimPrefix(key, "./")
	key = strings.TrimLeft(key, "/")
	cleaned := filepath.Clean(key)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	if cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("storage: invalid key")
	}
	return cleaned, nil
}
