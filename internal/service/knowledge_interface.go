package service

import (
	"mime/multipart"

	dto "docmind/internal/model/dto/response"
)

type KnowledgePreviewFile struct {
	FileName    string
	FileType    string
	ContentType string
	Content     []byte
}

// KnowledgeService 知识条目服务接口
type KnowledgeService interface {
	UploadFile(userID uint, knowledgeBaseID uint, fileHeader *multipart.FileHeader) (*dto.KnowledgeFileUploadResponse, error)
	PreviewFile(userID uint, knowledgeID uint) (*KnowledgePreviewFile, error)
}
