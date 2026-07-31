package service

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"path"
	"strings"
	"sync"

	"docmind/pkg/config"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type noopImageStorageService struct{}

func NewNoopImageStorageService() ImageStorageService {
	return &noopImageStorageService{}
}

func (s *noopImageStorageService) Enabled() bool {
	return false
}

func (s *noopImageStorageService) StoreDocumentImage(_ context.Context, _ *StoreDocumentImageRequest) (*StoredImage, error) {
	return nil, nil
}

type minioImageStorageService struct {
	client           *minio.Client
	cfg              config.MinIOConfig
	baseURL          string
	ensureBucketOnce sync.Once
	ensureBucketErr  error
}

func NewImageStorageService(cfg config.MinIOConfig) (ImageStorageService, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.BucketName) == "" {
		return NewNoopImageStorageService(), nil
	}

	client, err := minio.New(strings.TrimSpace(cfg.Endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(cfg.AccessKeyID), strings.TrimSpace(cfg.AccessKeySecret), ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &minioImageStorageService{
		client:  client,
		cfg:     cfg,
		baseURL: resolveMinIOBaseURL(cfg),
	}, nil
}

func (s *minioImageStorageService) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *minioImageStorageService) StoreDocumentImage(ctx context.Context, req *StoreDocumentImageRequest) (*StoredImage, error) {
	if req == nil || len(req.Data) == 0 {
		return nil, nil
	}
	if err := s.ensureBucket(ctx); err != nil {
		return nil, err
	}

	objectKey := s.buildObjectKey(req)
	contentType := strings.TrimSpace(req.MimeType)
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(req.Filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.client.PutObject(ctx, s.cfg.BucketName, objectKey, bytes.NewReader(req.Data), int64(len(req.Data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, err
	}

	return &StoredImage{
		ObjectKey:   objectKey,
		URL:         joinObjectURL(s.baseURL, s.cfg.BucketName, objectKey),
		Filename:    req.Filename,
		OriginalRef: req.OriginalRef,
		MimeType:    contentType,
	}, nil
}

func (s *minioImageStorageService) ensureBucket(ctx context.Context) error {
	s.ensureBucketOnce.Do(func() {
		exists, err := s.client.BucketExists(ctx, s.cfg.BucketName)
		if err != nil {
			s.ensureBucketErr = err
			return
		}
		if exists {
			return
		}
		s.ensureBucketErr = s.client.MakeBucket(ctx, s.cfg.BucketName, minio.MakeBucketOptions{})
	})
	return s.ensureBucketErr
}

func (s *minioImageStorageService) buildObjectKey(req *StoreDocumentImageRequest) string {
	if key := sanitizeObjectKey(req.SuggestedObjectKey); key != "" {
		return key
	}

	fileName := strings.TrimSpace(req.Filename)
	if fileName == "" {
		fileName = uuid.NewString() + defaultImageExtension(req.MimeType)
	}

	segments := make([]string, 0, 5)
	if prefix := sanitizeObjectKey(s.cfg.PathPrefix); prefix != "" {
		segments = append(segments, prefix)
	}
	segments = append(segments,
		fmt.Sprintf("kb-%d", req.KnowledgeBaseID),
		fmt.Sprintf("knowledge-%d", req.KnowledgeID),
		sanitizeObjectSegment(req.RequestID),
		sanitizeObjectSegment(fileName),
	)
	return path.Join(segments...)
}

func resolveMinIOBaseURL(cfg config.MinIOConfig) string {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL != "" {
		return baseURL
	}
	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, strings.TrimSpace(cfg.Endpoint))
}

func joinObjectURL(baseURL, bucketName, objectKey string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return fmt.Sprintf("%s/%s/%s", baseURL, strings.TrimSpace(bucketName), strings.TrimLeft(objectKey, "/"))
}

func sanitizeObjectKey(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizeObjectSegment(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return path.Join(cleaned...)
}

func sanitizeObjectSegment(value string) string {
	replacer := strings.NewReplacer("\\", "-", "/", "-", " ", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	value = strings.TrimSpace(replacer.Replace(value))
	value = strings.Trim(value, ".-")
	if value == "" {
		return "asset"
	}
	return value
}

func defaultImageExtension(mimeType string) string {
	extensions, err := mime.ExtensionsByType(strings.TrimSpace(mimeType))
	if err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ".bin"
}
