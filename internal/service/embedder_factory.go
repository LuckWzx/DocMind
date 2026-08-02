package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"
	"docmind/internal/repository"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	einoembedding "github.com/cloudwego/eino/components/embedding"
)

// EmbedderFactory 根据数据库中的模型配置创建 eino Embedder 实例
type EmbedderFactory struct {
	modelRepo repository.ModelRepository
}

// NewEmbedderFactory 创建 Embedder 工厂
func NewEmbedderFactory(modelRepo repository.ModelRepository) *EmbedderFactory {
	return &EmbedderFactory{modelRepo: modelRepo}
}

// CreateEmbedder 根据 Embedding 模型 ID 创建 eino Embedder
func (f *EmbedderFactory) CreateEmbedder(ctx context.Context, modelID string) (einoembedding.Embedder, error) {
	model, err := f.resolveModel(modelID)
	if err != nil {
		return nil, err
	}

	modelName := strings.TrimSpace(model.Parameters.ModelName)
	if modelName == "" {
		modelName = model.Name
	}

	cfg := &openai.EmbeddingConfig{
		APIKey:  model.Parameters.APIKey,
		Model:   modelName,
		BaseURL: model.Parameters.BaseURL,
	}

	if model.Parameters.EmbeddingParameters.Dimension > 0 {
		dim := model.Parameters.EmbeddingParameters.Dimension
		cfg.Dimensions = &dim
	}

	return openai.NewEmbedder(ctx, cfg)
}

// CreatePipelineEmbedder 适配 pipeline.PipelineEmbedderFactory 接口
// 将 einoembedding.Embedder（返回 [][]float64）适配为 PipelineEmbedder（返回 []float64，单条文本）
func (f *EmbedderFactory) CreatePipelineEmbedder(ctx context.Context, modelID string) (pipeline.PipelineEmbedder, error) {
	embedder, err := f.CreateEmbedder(ctx, modelID)
	if err != nil {
		return nil, err
	}
	return &pipelineEmbedderAdapter{inner: embedder}, nil
}

// pipelineEmbedderAdapter 适配器：将 eino Embedder 适配为 pipeline.PipelineEmbedder
type pipelineEmbedderAdapter struct {
	inner einoembedding.Embedder
}

func (a *pipelineEmbedderAdapter) EmbedStrings(ctx context.Context, texts []string) ([]float64, error) {
	results, err := a.inner.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Embedding 返回空结果")
	}
	return results[0], nil
}

// resolveModel 根据模型 ID 查找 Embedding 模型记录
func (f *EmbedderFactory) resolveModel(modelID string) (*entity.Model, error) {
	if id, err := strconv.ParseUint(modelID, 10, 64); err == nil {
		model, err := f.modelRepo.FindByID(uint(id))
		if err != nil {
			return nil, fmt.Errorf("查询模型失败: %w", err)
		}
		if model != nil {
			return model, nil
		}
	}
	models, err := f.modelRepo.List(entity.ModelTypeEmbedding, 0)
	if err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	for _, m := range models {
		if m.Name == modelID || m.DisplayName == modelID {
			return m, nil
		}
	}
	// 如果没指定模型，尝试使用默认模型
	for _, m := range models {
		if m.IsDefault {
			return m, nil
		}
	}
	return nil, fmt.Errorf("未找到 Embedding 模型: %s", modelID)
}
