package service

import (
	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
)

// ChunkerService Markdown 分块服务接口
type ChunkerService interface {
	Preview(request *req.PreviewChunkingRequest) (*dto.PreviewChunkingResponse, error)
}
