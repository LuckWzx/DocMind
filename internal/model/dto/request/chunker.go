package request

import "docmind/internal/model/entity"

// PreviewChunkingRequest 分块预览请求
type PreviewChunkingRequest struct {
	Text           string                `json:"text" binding:"required"`
	ChunkingConfig entity.ChunkingConfig `json:"chunking_config"`
}
