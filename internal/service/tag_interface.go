package service

import (
	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

type TagService interface {
	List(userID, kbID uint, request *req.TagListRequest) ([]*dto.TagResponse, int64, error)
	Create(userID, kbID uint, request *req.CreateTagRequest) (*dto.TagResponse, error)
	Update(userID, kbID, id uint, request *req.UpdateTagRequest) (*dto.TagResponse, error)
	Delete(userID, kbID, id uint, force bool) error
}
