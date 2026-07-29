package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

type knowledgePipelineGatewayMock struct{}

func NewKnowledgePipelineGatewayMock() KnowledgePipelineGateway {
	return &knowledgePipelineGatewayMock{}
}

func (g *knowledgePipelineGatewayMock) StageUpload(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	_ = ctx
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	target := filepath.Join(os.TempDir(), "docmind-kb-mock-"+filepath.Base(fileHeader.Filename))
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return target, nil
}

func (g *knowledgePipelineGatewayMock) ParseDocument(ctx context.Context, sourceRef string, processConfig string) (*ParsedDocument, error) {
	_ = ctx
	_ = processConfig
	raw, err := os.ReadFile(sourceRef)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(string(raw))
	if text == "" {
		text = "Mock parsed content from uploaded file."
	}

	paragraphs := strings.Split(text, "\n\n")
	chunks := make([]string, 0, len(paragraphs))
	for _, item := range paragraphs {
		item = strings.TrimSpace(item)
		if item != "" {
			chunks = append(chunks, item)
		}
	}
	if len(chunks) == 0 {
		chunks = []string{text}
	}

	return &ParsedDocument{
		Title:  strings.TrimSuffix(filepath.Base(sourceRef), filepath.Ext(sourceRef)),
		Chunks: chunks,
	}, nil
}

func (g *knowledgePipelineGatewayMock) BuildEmbeddings(ctx context.Context, modelID string, texts []string) ([][]float32, error) {
	_ = ctx
	_ = modelID
	vectors := make([][]float32, 0, len(texts))
	for idx, text := range texts {
		size := float32(len([]rune(text)))
		vectors = append(vectors, []float32{
			float32(idx + 1),
			size,
			size / 10,
			float32(len(text) % 17),
			1,
			0,
			0,
			1,
		})
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("没有可生成向量的文本")
	}
	return vectors, nil
}
