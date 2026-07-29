package service

import (
	"mime/multipart"

	dto "docmind/internal/model/dto/response"
)

// KnowledgeService 知识条目服务接口
type KnowledgeService interface {
	UploadFile(userID uint, knowledgeBaseID uint, fileHeader *multipart.FileHeader) (*dto.KnowledgeFileUploadResponse, error)
}
