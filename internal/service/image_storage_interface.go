package service

import "context"

// StoreDocumentImageRequest 文档图片存储请求
type StoreDocumentImageRequest struct {
	KnowledgeBaseID    uint
	KnowledgeID        uint
	RequestID          string
	Filename           string
	OriginalRef        string
	MimeType           string
	SuggestedObjectKey string
	Data               []byte
}

// StoredImage 文档图片存储结果
type StoredImage struct {
	ObjectKey   string `json:"object_key,omitempty"`
	URL         string `json:"url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	OriginalRef string `json:"original_ref,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
}

// ImageStorageService 图片存储服务
type ImageStorageService interface {
	Enabled() bool
	StoreDocumentImage(ctx context.Context, req *StoreDocumentImageRequest) (*StoredImage, error)
}
