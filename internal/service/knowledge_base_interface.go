package service

import (
	"mime/multipart"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

type KnowledgeBaseService interface {
	List(userID uint) ([]*dto.KnowledgeBaseResponse, error)
	Create(userID uint, request *req.CreateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error)
	Get(userID, id uint) (*dto.KnowledgeBaseResponse, error)
	Update(userID, id uint, request *req.UpdateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error)
	Delete(userID, id uint) error

	UploadFile(userID, kbID uint, fileHeader *multipart.FileHeader, processConfig string, tagID *uint) (*dto.KnowledgeResponse, error)
	ListKnowledge(userID, kbID uint, request *req.KnowledgeListRequest) ([]*dto.KnowledgeResponse, int64, error)
	GetKnowledge(userID, id uint) (*dto.KnowledgeDetailResponse, error)
	ListKnowledgeChunks(userID, knowledgeID uint, page, pageSize int) ([]map[string]interface{}, int64, error)
	UpdateKnowledgeTags(userID uint, updates map[uint][]uint) error
	ReparseKnowledge(userID, id uint, processConfig string) (*dto.KnowledgeResponse, error)
	DeleteKnowledge(userID, id uint) error
}
