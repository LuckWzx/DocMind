package service

import (
	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

type FAQService interface {
	List(userID, kbID uint, request *req.FAQListRequest) ([]*dto.FAQResponse, int64, error)
	Create(userID, kbID uint, request *req.FAQEntryUpsertRequest) (*dto.FAQResponse, error)
	Update(userID, kbID, id uint, request *req.FAQEntryUpsertRequest) (*dto.FAQResponse, error)
	BatchUpsert(userID, kbID uint, request *req.FAQEntriesUpsertRequest) error
	BatchUpdateFields(userID, kbID uint, request *req.FAQEntryFieldsBatchRequest) error
	DeleteBatch(userID, kbID uint, ids []uint) error
	Export(userID, kbID uint) ([]byte, error)
}
