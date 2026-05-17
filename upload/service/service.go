package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	minioclient "rabhana/lib/minio"
	"rabhana/pkg/errs"
)

type UploadService struct {
	minio        *minioclient.Client
	maxFileSize  int64
	allowedTypes []string
}

func NewUploadService(minio *minioclient.Client, maxFileSizeMB int) *UploadService {
	if maxFileSizeMB == 0 {
		maxFileSizeMB = 10
	}
	return &UploadService{
		minio:        minio,
		maxFileSize:  int64(maxFileSizeMB) * 1024 * 1024,
		allowedTypes: []string{".jpg", ".jpeg", ".png", ".pdf"},
	}
}

// UploadFile uploads to the public bucket and returns a permanent URL.
// This satisfies the UploadService interface used by auction services.
func (s *UploadService) UploadFile(ctx context.Context, file []byte, originalName string) (string, error) {
	if s.minio == nil {
		return "", errs.ErrStorageUnavailable
	}
	if err := s.validate(file, originalName); err != nil {
		return "", err
	}

	objectKey := s.objectKey(originalName)
	contentType := contentTypeFor(originalName)

	return s.minio.UploadPublic(ctx, objectKey, file, contentType)
}

// UploadPrivateFile uploads to the private bucket and returns the object key (not a URL).
func (s *UploadService) UploadPrivateFile(ctx context.Context, file []byte, originalName string) (string, error) {
	if s.minio == nil {
		return "", errs.ErrStorageUnavailable
	}
	if err := s.validate(file, originalName); err != nil {
		return "", err
	}

	objectKey := s.objectKey(originalName)
	contentType := contentTypeFor(originalName)

	return s.minio.UploadPrivate(ctx, objectKey, file, contentType)
}

// GetPresignedURL generates a time-limited signed URL for a private object.
func (s *UploadService) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if s.minio == nil {
		return "", errs.ErrStorageUnavailable
	}
	return s.minio.GetPresignedURL(ctx, objectKey, expiry)
}

func (s *UploadService) validate(file []byte, originalName string) error {
	if int64(len(file)) > s.maxFileSize {
		return errs.ErrFileTooLarge
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if !s.isAllowedType(ext) {
		return errs.ErrInvalidFileType
	}
	return nil
}

func (s *UploadService) isAllowedType(ext string) bool {
	for _, allowed := range s.allowedTypes {
		if ext == allowed {
			return true
		}
	}
	return false
}

func (s *UploadService) objectKey(originalName string) string {
	ext := strings.ToLower(filepath.Ext(originalName))
	now := time.Now()
	return fmt.Sprintf("%d/%02d/%02d/%s%s", now.Year(), now.Month(), now.Day(), uuid.New().String(), ext)
}

func contentTypeFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}
