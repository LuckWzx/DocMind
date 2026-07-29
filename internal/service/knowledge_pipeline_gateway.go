package service

import (
	"context"
	"mime/multipart"
)

type ParsedDocument struct {
	Title  string
	Chunks []string
}

type KnowledgePipelineGateway interface {
	StageUpload(ctx context.Context, fileHeader *multipart.FileHeader) (string, error)
	ParseDocument(ctx context.Context, sourceRef string, processConfig string) (*ParsedDocument, error)
	BuildEmbeddings(ctx context.Context, modelID string, texts []string) ([][]float32, error)
}
