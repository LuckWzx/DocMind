package service

import (
	"context"
	"fmt"
	"strconv"

	"docmind/internal/model/entity"
	"docmind/internal/repository"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

// ChatModelFactory 根据数据库中的模型配置创建 eino ChatModel 实例
type ChatModelFactory struct {
	modelRepo repository.ModelRepository
}

// NewChatModelFactory 创建 ChatModel 工厂
func NewChatModelFactory(modelRepo repository.ModelRepository) *ChatModelFactory {
	return &ChatModelFactory{modelRepo: modelRepo}
}

// CreateChatModel 根据模型 ID 创建 eino ChatModel
func (f *ChatModelFactory) CreateChatModel(ctx context.Context, modelID string) (einomodel.ToolCallingChatModel, error) {
	model, err := f.resolveModel(modelID)
	if err != nil {
		return nil, err
	}

	cfg := &openai.ChatModelConfig{
		APIKey:  model.Parameters.APIKey,
		Model:   model.Parameters.ModelName,
		BaseURL: model.Parameters.BaseURL,
	}

	if model.Parameters.Temperature > 0 {
		temp := float32(model.Parameters.Temperature)
		cfg.Temperature = &temp
	}
	if model.Parameters.MaxTokens > 0 {
		cfg.MaxCompletionTokens = &model.Parameters.MaxTokens
	}

	return openai.NewChatModel(ctx, cfg)
}

// resolveModel 根据模型 ID（可以是数字 ID 字符串或模型名称）查找模型记录
func (f *ChatModelFactory) resolveModel(modelID string) (*entity.Model, error) {
	// 尝试按数字 ID 查找
	if id, err := strconv.ParseUint(modelID, 10, 64); err == nil {
		model, err := f.modelRepo.FindByID(uint(id))
		if err != nil {
			return nil, fmt.Errorf("查询模型失败: %w", err)
		}
		if model != nil {
			return model, nil
		}
	}
	// 尝试按名称查找（使用 userID=0 查全局模型）
	models, err := f.modelRepo.List(entity.ModelTypeKnowledgeQA, 0)
	if err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	for _, m := range models {
		if m.Name == modelID || m.DisplayName == modelID {
			return m, nil
		}
	}
	return nil, fmt.Errorf("未找到模型: %s", modelID)
}
