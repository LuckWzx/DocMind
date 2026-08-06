package pipeline

import (
	"context"
	"fmt"
	"strconv"

	"docmind/internal/model/entity"
	"docmind/internal/repository"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

// ChatCompletionNode LLM 调用节点
func chatCompletionNode(ctx context.Context, input *Context) (*Context, error) {
	// 1. 创建 ChatModel
	chatModel, err := createChatModel(ctx, input.ModelRepo, input.AgentConfig.ModelID, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}

	// 2. 流式调用
	stream, err := chatModel.Stream(ctx, input.Messages)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}

	input.Stream = stream

	fmt.Printf("[Pipeline] ChatCompletion: modelID=%s, messages=%d\n", input.AgentConfig.ModelID, len(input.Messages))

	return input, nil
}

// createChatModel 创建 ChatModel
func createChatModel(ctx context.Context, modelRepo repository.ModelRepository, modelID string, userID uint) (einomodel.ToolCallingChatModel, error) {
	var model *entity.Model
	var err error

	// 如果是 "default"，查找默认模型
	if modelID == "default" || modelID == "" {
		// 优先查找当前用户自己的 KnowledgeQA 模型
		models, listErr := modelRepo.List(entity.ModelTypeKnowledgeQA, userID)
		if listErr != nil {
			return nil, fmt.Errorf("查询模型列表失败: %w", listErr)
		}
		// 当前用户未配置，回退到系统级默认模型（user_id=0），避免跨用户使用他人模型
		if len(models) == 0 {
			models, listErr = modelRepo.List(entity.ModelTypeKnowledgeQA, 0)
			if listErr != nil {
				return nil, fmt.Errorf("查询模型列表失败: %w", listErr)
			}
		}
		// 查找默认模型
		for _, m := range models {
			if m.IsDefault {
				model = m
				break
			}
		}
		// 如果没有默认模型，使用第一个
		if model == nil && len(models) > 0 {
			model = models[0]
		}
	} else {
		// 按数字 ID 查找
		id, parseErr := strconv.ParseUint(modelID, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("无效的模型 ID: %s", modelID)
		}
		model, err = modelRepo.FindByID(uint(id))
		if err != nil {
			return nil, fmt.Errorf("查询模型失败: %w", err)
		}
	}

	if model == nil {
		return nil, fmt.Errorf("未找到模型: %s", modelID)
	}

	// 打印模型信息用于调试
	fmt.Printf("[Pipeline] Model: id=%d, name=%s, displayName=%s, modelName=%s, baseURL=%s\n",
		model.ID, model.Name, model.DisplayName, model.Parameters.ModelName, model.Parameters.BaseURL)

	// 确定模型名称：优先使用 modelName，其次使用 name
	modelName := model.Parameters.ModelName
	if modelName == "" {
		modelName = model.Name
	}

	// 创建 ChatModel
	cfg := &openai.ChatModelConfig{
		APIKey:  model.Parameters.APIKey,
		Model:   modelName,
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
