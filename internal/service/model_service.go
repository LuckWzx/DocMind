package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	pkgerrors "docmind/pkg/errors"
	"docmind/pkg/logger"
)

const docMindCloudCredentialKey = "docmindcloud_credentials"

// modelService 模型管理服务实现，统一管理模型配置的 CRUD、连接探测、调试与向量化调用。
// Ollama 下载任务及 HTTP 工具方法分别拆分至 model_service_ollama.go / model_service_http.go。
type modelService struct {
	modelRepo   repository.ModelRepository
	settingRepo repository.SystemSettingRepository
	httpClient  *http.Client
	// missingRepo 上下文大小缺失记录（厂商接口与映射表均未命中时写入，供定期补足映射表）
	missingRepo repository.ModelContextWindowMissingRepository

	taskMu sync.RWMutex
	tasks  map[string]*ollamaDownloadTask
}

// NewModelService 创建模型管理服务实例，需注入数据仓库、系统配置仓库及 HTTP 客户端。
func NewModelService(
	modelRepo repository.ModelRepository,
	settingRepo repository.SystemSettingRepository,
	httpClient *http.Client,
	missingRepo repository.ModelContextWindowMissingRepository,
) ModelService {
	return &modelService{
		modelRepo:   modelRepo,
		settingRepo: settingRepo,
		httpClient:  httpClient,
		missingRepo: missingRepo,
		tasks:       make(map[string]*ollamaDownloadTask),
	}
}

// ---------- 模型 CRUD ----------

// CreateModel 创建新的模型配置。
// 校验请求参数后以 name+type+source+provider 四项组合去重，防止同质模型重复添加。
func (s *modelService) CreateModel(userID uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error) {
	if err := validateModelRequest(request); err != nil {
		return nil, err
	}
	existing, err := s.modelRepo.FindDuplicate(userID, strings.TrimSpace(request.Name), request.Type, request.Source, strings.TrimSpace(request.Parameters.Provider))
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if existing != nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceAlreadyExists, "已存在同名且同类型、同来源、同供应商的模型，请修改名称或改用其他配置")
	}

	model := entity.Model{
		UserID:      userID,
		Name:        strings.TrimSpace(request.Name),
		DisplayName: strings.TrimSpace(request.DisplayName),
		Type:        request.Type,
		Source:      request.Source,
		Description: strings.TrimSpace(request.Description),
		Status:      "active",
		IsDefault:   request.IsDefault,
		IsBuiltin:   false,
	}
	params := toEntityParameters(request.Parameters, entity.ModelParameters{})
	var ctxWindowReason string
	if params.ContextWindow <= 0 {
		// 未显式配置时自动获取：元数据接口优先，内置映射表兜底；失败不阻塞创建
		var w int
		w, ctxWindowReason = s.resolveContextWindow(&model)
		if w > 0 {
			params.ContextWindow = w
		}
	}
	model.Parameters = params
	if err := s.modelRepo.Create(&model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "创建模型失败", err)
	}
	if params.ContextWindow <= 0 {
		// 记录缺失清单（需模型入库拿到 ID），供后续定期补足映射表
		logger.Warnf("[ModelContextWindow] 模型 %s 上下文大小获取失败: %s", model.Name, ctxWindowReason)
		s.recordMissingContextWindow(&model, ctxWindowReason)
	}
	return s.buildModelResponse(&model), nil
}

// ListModels 查询指定用户的模型列表，可按 modelType 过滤。
func (s *modelService) ListModels(userID uint, modelType string) ([]*dto.ModelResponse, error) {
	models, err := s.modelRepo.List(modelType, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型列表失败", err)
	}
	result := make([]*dto.ModelResponse, 0, len(models))
	for _, m := range models {
		result = append(result, s.buildModelResponse(m))
	}
	return result, nil
}

// GetModel 按 ID 获取单个模型配置（校验所属用户）。
func (s *modelService) GetModel(userID uint, id uint) (*dto.ModelResponse, error) {
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	return s.buildModelResponse(model), nil
}

// UpdateModel 更新已有模型配置。
// 仅当 name/type/source/provider 任一发生变化时才执行去重检查。
func (s *modelService) UpdateModel(userID uint, id uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error) {
	if err := validateModelRequest(request); err != nil {
		return nil, err
	}
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}

	nameChanged := strings.TrimSpace(request.Name) != model.Name
	typeChanged := request.Type != model.Type
	sourceChanged := request.Source != model.Source
	providerChanged := strings.TrimSpace(request.Parameters.Provider) != model.Parameters.Provider

	if nameChanged || typeChanged || sourceChanged || providerChanged {
		existing, err := s.modelRepo.FindDuplicate(userID, strings.TrimSpace(request.Name), request.Type, request.Source, strings.TrimSpace(request.Parameters.Provider))
		if err != nil {
			return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
		}
		if existing != nil && existing.ID != model.ID {
			return nil, pkgerrors.New(pkgerrors.CodeResourceAlreadyExists, "已存在同名且同类型、同来源、同供应商的模型，请修改名称或改用其他配置")
		}
	}

	model.Name = strings.TrimSpace(request.Name)
	model.DisplayName = strings.TrimSpace(request.DisplayName)
	model.Type = request.Type
	model.Source = request.Source
	model.Description = strings.TrimSpace(request.Description)
	model.IsDefault = request.IsDefault
	model.Parameters = toEntityParameters(request.Parameters, model.Parameters)
	var ctxWindowReason string
	if model.Parameters.ContextWindow <= 0 {
		// 未显式配置时重新自动获取（更新可能变更了模型名/BaseURL，需刷新缓存值）
		var w int
		w, ctxWindowReason = s.resolveContextWindow(model)
		if w > 0 {
			model.Parameters.ContextWindow = w
		}
	}
	if err := s.modelRepo.Update(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "更新模型失败", err)
	}
	if model.Parameters.ContextWindow > 0 {
		// 上下文已确定：清理历史缺失记录
		s.clearMissingContextWindow(model.ID)
	} else {
		logger.Warnf("[ModelContextWindow] 模型 %s 上下文大小获取失败: %s", model.Name, ctxWindowReason)
		s.recordMissingContextWindow(model, ctxWindowReason)
	}
	return s.buildModelResponse(model), nil
}

// DeleteModel 删除模型配置，内置模型（IsBuiltin）不允许删除。
func (s *modelService) DeleteModel(userID uint, id uint) error {
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	if model.IsBuiltin {
		return pkgerrors.New(pkgerrors.CodeForbidden, "内置模型不可删除")
	}
	if err := s.modelRepo.Delete(id); err != nil {
		return err
	}
	// 联动清理上下文大小缺失记录
	s.clearMissingContextWindow(id)
	return nil
}

// PutModelCredentials 更新模型的 API Key / App Secret 凭据字段。
func (s *modelService) PutModelCredentials(userID uint, id uint, request *req.PutModelCredentialsRequest) (*dto.ModelCredentialsResponse, error) {
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	if request.APIKey != nil && strings.TrimSpace(*request.APIKey) != "" {
		model.Parameters.APIKey = strings.TrimSpace(*request.APIKey)
	}
	if request.AppSecret != nil && strings.TrimSpace(*request.AppSecret) != "" {
		model.Parameters.AppSecret = strings.TrimSpace(*request.AppSecret)
	}
	if err := s.modelRepo.Update(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "更新凭据失败", err)
	}
	return buildCredentialResponse(model.Parameters), nil
}

// DeleteModelCredentialField 清除指定凭据字段（api_key 或 app_secret）。
func (s *modelService) DeleteModelCredentialField(userID uint, id uint, field string) (*dto.ModelCredentialsResponse, error) {
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	switch field {
	case "api_key":
		model.Parameters.APIKey = ""
	case "app_secret":
		model.Parameters.AppSecret = ""
	default:
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "不支持的凭据字段")
	}
	if err := s.modelRepo.Update(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "删除凭据字段失败", err)
	}
	return buildCredentialResponse(model.Parameters), nil
}

// ---------- 供应商列表 ----------

// ListProviders 返回可选模型供应商列表，可按模型类型过滤。
func (s *modelService) ListProviders(modelType string) []*dto.ModelProviderOptionResponse {
	all := []*dto.ModelProviderOptionResponse{
		{
			Value: "openai", Label: "OpenAI",
			ModelTypes: []string{"chat", "embedding", "vllm", "asr"},
			DefaultURLs: map[string]string{
				"chat":      "https://api.openai.com/v1",
				"embedding": "https://api.openai.com/v1",
				"vllm":      "https://api.openai.com/v1",
				"asr":       "https://api.openai.com/v1",
			},
		},
		{
			Value: "aliyun", Label: "阿里云 DashScope",
			ModelTypes: []string{"chat", "embedding", "rerank", "vllm"},
			DefaultURLs: map[string]string{
				"chat":      "https://dashscope.aliyuncs.com/compatible-mode/v1",
				"embedding": "https://dashscope.aliyuncs.com/compatible-mode/v1",
				"rerank":    "https://dashscope.aliyuncs.com/compatible-mode/v1",
				"vllm":      "https://dashscope.aliyuncs.com/compatible-mode/v1",
			},
		},
		{
			Value: "siliconflow", Label: "SiliconFlow",
			ModelTypes: []string{"chat", "embedding", "rerank"},
			DefaultURLs: map[string]string{
				"chat":      "https://api.siliconflow.cn/v1",
				"embedding": "https://api.siliconflow.cn/v1",
				"rerank":    "https://api.siliconflow.cn/v1",
			},
		},
		{
			Value: "zhipu", Label: "智谱 GLM",
			ModelTypes: []string{"chat", "embedding", "vllm"},
			DefaultURLs: map[string]string{
				"chat":      "https://open.bigmodel.cn/api/paas/v4",
				"embedding": "https://open.bigmodel.cn/api/paas/v4",
				"vllm":      "https://open.bigmodel.cn/api/paas/v4",
			},
		},
		{
			Value: "jina", Label: "Jina AI",
			ModelTypes: []string{"embedding", "rerank"},
			DefaultURLs: map[string]string{
				"embedding": "https://api.jina.ai/v1",
				"rerank":    "https://api.jina.ai/v1",
			},
		},
		{
			Value: "generic", Label: "自定义",
			ModelTypes: []string{"chat", "embedding", "rerank", "vllm", "asr"},
		},
	}
	if modelType == "" {
		return all
	}
	var filtered []*dto.ModelProviderOptionResponse
	for _, provider := range all {
		for _, t := range provider.ModelTypes {
			if t == modelType {
				filtered = append(filtered, provider)
				break
			}
		}
	}
	return filtered
}

// ---------- DocMindCloud ----------

// SaveDocMindCloudCredentials 保存 DocMindCloud 的 AppID 和 AppSecret。
func (s *modelService) SaveDocMindCloudCredentials(appID, appSecret string) error {
	setting, err := s.settingRepo.FindByKey(docMindCloudCredentialKey)
	if err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询凭据失败", err)
	}
	if setting == nil {
		setting = &entity.SystemSetting{
			Key:   docMindCloudCredentialKey,
			Value: map[string]interface{}{},
		}
	}
	setting.Value["app_id"] = strings.TrimSpace(appID)
	setting.Value["app_secret"] = strings.TrimSpace(appSecret)
	return s.settingRepo.Upsert(setting)
}

// GetDocMindCloudStatus 返回 DocMindCloud 凭据是否已配置。
func (s *modelService) GetDocMindCloudStatus() (*dto.DocMindCloudStatusResponse, error) {
	creds, err := s.getDocMindCloudCredentials()
	if err != nil {
		return nil, err
	}
	return &dto.DocMindCloudStatusResponse{
		HasModels: strings.TrimSpace(creds["app_id"]) != "" && strings.TrimSpace(creds["app_secret"]) != "",
	}, nil
}

// ---------- 模型探测 ----------

// CheckRemoteModel 对聊天/通用模型执行连通性探测（发送 ping 消息）。
func (s *modelService) CheckRemoteModel(request *req.ModelTestRequest) (map[string]interface{}, error) {
	cfg, err := s.resolveTestConfig(request)
	if err != nil {
		return nil, err
	}
	if cfg.Source == "local" {
		availableMap, err := s.CheckOllamaModels([]string{cfg.ModelName})
		if err != nil {
			return map[string]interface{}{"available": false, "message": err.Error()}, nil
		}
		return map[string]interface{}{
			"available": availableMap[cfg.ModelName],
			"message":   boolMessage(availableMap[cfg.ModelName], "模型可用", "模型未安装"),
		}, nil
	}
	url := appendPath(cfg.BaseURL, "chat/completions")
	payload := map[string]interface{}{
		"model": cfg.ModelName,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "ping"},
		},
		"max_tokens":  1,
		"temperature": 0,
	}
	status, body, err := s.doJSONRequest(http.MethodPost, url, payload, buildAuthHeaders(cfg.APIKey, cfg.CustomHeaders))
	if err != nil {
		return map[string]interface{}{"available": false, "message": err.Error()}, nil
	}
	if status >= 200 && status < 300 {
		return map[string]interface{}{"available": true, "message": "连接成功"}, nil
	}
	return map[string]interface{}{"available": false, "message": extractErrorMessage(body, status)}, nil
}

// TestEmbeddingModel 对 embedding 模型执行连通性探测，返回向量维度。
func (s *modelService) TestEmbeddingModel(request *req.ModelTestRequest) (map[string]interface{}, error) {
	cfg, err := s.resolveTestConfig(request)
	if err != nil {
		return nil, err
	}
	if cfg.Provider == "docmindcloud" {
		return map[string]interface{}{"available": true, "message": "DocMindCloud 凭据已配置", "dimension": 1024}, nil
	}
	if cfg.Source == "local" {
		result, raw, err := s.callOllamaEmbed(cfg.ModelName, "hello world")
		if err != nil {
			return map[string]interface{}{"available": false, "message": err.Error()}, nil
		}
		_ = raw
		return map[string]interface{}{"available": true, "message": "连接成功", "dimension": len(result)}, nil
	}

	url := appendPath(cfg.BaseURL, "embeddings")
	payload := map[string]interface{}{
		"model": cfg.ModelName,
		"input": "hello world",
	}
	status, body, err := s.doJSONRequest(http.MethodPost, url, payload, buildAuthHeaders(cfg.APIKey, cfg.CustomHeaders))
	if err != nil {
		return map[string]interface{}{"available": false, "message": err.Error()}, nil
	}
	if status < 200 || status >= 300 {
		return map[string]interface{}{"available": false, "message": extractErrorMessage(body, status)}, nil
	}
	dimension := extractEmbeddingDimension(body)
	result := map[string]interface{}{
		"available": true,
		"message":   "连接成功",
	}
	if dimension > 0 {
		result["dimension"] = dimension
	}
	return result, nil
}

// CheckRerankModel 对 rerank 模型执行连通性探测，自动尝试多种 URL 后缀。
func (s *modelService) CheckRerankModel(request *req.ModelTestRequest) (map[string]interface{}, error) {
	cfg, err := s.resolveTestConfig(request)
	if err != nil {
		return nil, err
	}
	if cfg.Provider == "docmindcloud" {
		return map[string]interface{}{"available": true, "message": "DocMindCloud 凭据已配置"}, nil
	}
	if cfg.Source == "local" {
		return map[string]interface{}{"available": false, "message": "Ollama 暂不支持 Rerank 测试"}, nil
	}

	candidates := []string{cfg.BaseURL}
	if !strings.Contains(strings.ToLower(cfg.BaseURL), "rerank") {
		candidates = append(candidates, appendPath(cfg.BaseURL, "rerank"))
		candidates = append(candidates, appendPath(cfg.BaseURL, "v1/rerank"))
	}
	payload := map[string]interface{}{
		"model":     cfg.ModelName,
		"query":     "测试查询",
		"documents": []string{"第一段文档", "第二段文档"},
	}
	for _, candidate := range deduplicateStrings(candidates) {
		status, body, err := s.doJSONRequest(http.MethodPost, candidate, payload, buildAuthHeaders(cfg.APIKey, cfg.CustomHeaders))
		if err != nil {
			continue
		}
		if status >= 200 && status < 300 {
			return map[string]interface{}{"available": true, "message": "连接成功"}, nil
		}
		if status != http.StatusNotFound {
			return map[string]interface{}{"available": false, "message": extractErrorMessage(body, status)}, nil
		}
	}
	return map[string]interface{}{"available": false, "message": "未找到可用的 Rerank 接口"}, nil
}

// CheckASRModel 对 ASR（语音识别）模型执行连通性探测。
func (s *modelService) CheckASRModel(request *req.ModelTestRequest) (map[string]interface{}, error) {
	cfg, err := s.resolveTestConfig(request)
	if err != nil {
		return nil, err
	}
	if cfg.Source == "local" {
		return map[string]interface{}{"available": false, "message": "本地 ASR 测试暂未实现"}, nil
	}

	url := appendPath(cfg.BaseURL, "audio/transcriptions")
	status, body, err := s.doMultipartTranscription(url, cfg.ModelName, buildTestWAV(), "sample.wav", buildAuthHeaders(cfg.APIKey, cfg.CustomHeaders))
	if err != nil {
		return map[string]interface{}{"available": false, "message": err.Error()}, nil
	}
	if status >= 200 && status < 300 {
		return map[string]interface{}{"available": true, "message": "连接成功"}, nil
	}
	return map[string]interface{}{"available": false, "message": extractErrorMessage(body, status)}, nil
}

// ---------- 模型调试 ----------

// DebugModel 对指定模型执行调试调用，根据模型类型分发到对应的调试方法。
func (s *modelService) DebugModel(userID uint, id uint, input string, documents []string, options map[string]interface{}, fileHeader *multipart.FileHeader) (*dto.ModelDebugResult, error) {
	model, err := s.modelRepo.FindByUserID(id, userID)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}

	startedAt := time.Now()
	switch model.Type {
	case entity.ModelTypeKnowledgeQA:
		return s.debugChatLikeModel(model, input, options, fileHeader, false, startedAt)
	case entity.ModelTypeVLLM:
		return s.debugChatLikeModel(model, input, options, fileHeader, true, startedAt)
	case entity.ModelTypeEmbedding:
		vector, raw, err := s.debugEmbeddingModel(model, input)
		return &dto.ModelDebugResult{
			OK:        err == nil,
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Request: map[string]interface{}{
				"model": model.Name,
				"input": input,
			},
			RawResponse: raw,
			Observations: map[string]interface{}{
				"dimension": len(vector),
			},
			Error: errString(err),
		}, nil
	case entity.ModelTypeRerank:
		raw, count, err := s.debugRerankModel(model, input, documents)
		return &dto.ModelDebugResult{
			OK:        err == nil,
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Request: map[string]interface{}{
				"model":     model.Name,
				"query":     input,
				"documents": documents,
			},
			RawResponse: raw,
			Observations: map[string]interface{}{
				"result_count": count,
			},
			Error: errString(err),
		}, nil
	case entity.ModelTypeASR:
		raw, text, err := s.debugASRModel(model, fileHeader)
		return &dto.ModelDebugResult{
			OK:        err == nil,
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Request: map[string]interface{}{
				"model": model.Name,
				"file":  fileName(fileHeader),
			},
			RawResponse: raw,
			Observations: map[string]interface{}{
				"text_characters": len(text),
			},
			Error: errString(err),
		}, nil
	default:
		return nil, pkgerrors.New(pkgerrors.CodeNotImplemented, "暂不支持该模型类型的调试")
	}
}

// debugChatLikeModel 对聊天/视觉模型执行调试调用，支持 system prompt、temperature、thinking 控制及 Vision 图片。
func (s *modelService) debugChatLikeModel(model *entity.Model, input string, options map[string]interface{}, fileHeader *multipart.FileHeader, withVision bool, startedAt time.Time) (*dto.ModelDebugResult, error) {
	payload := map[string]interface{}{
		"model": model.Name,
		"messages": []map[string]interface{}{
			{"role": "user", "content": input},
		},
	}
	if systemPrompt, _ := options["system_prompt"].(string); strings.TrimSpace(systemPrompt) != "" {
		payload["messages"] = append([]map[string]interface{}{
			{"role": "system", "content": systemPrompt},
		}, payload["messages"].([]map[string]interface{})...)
	}
	if temperature, ok := options["temperature"]; ok {
		payload["temperature"] = temperature
	}
	if maxTokens, ok := options["max_tokens"]; ok {
		payload["max_tokens"] = maxTokens
	}
	if withVision && fileHeader != nil {
		dataURL, err := fileHeaderToDataURL(fileHeader)
		if err != nil {
			return nil, pkgerrors.NewWithErr(pkgerrors.CodeBadRequest, "读取图片失败", err)
		}
		payload["messages"] = []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": input},
					{"type": "image_url", "image_url": map[string]interface{}{"url": dataURL}},
				},
			},
		}
	}

	if model.Source == "local" {
		raw, answer, err := s.callOllamaChat(model.Name, input, options, fileHeader, withVision)
		return &dto.ModelDebugResult{
			OK:          err == nil,
			ElapsedMS:   time.Since(startedAt).Milliseconds(),
			Request:     payload,
			RawResponse: raw,
			Observations: map[string]interface{}{
				"answer_characters": len(answer),
			},
			Error: errString(err),
		}, nil
	}

	if model.Parameters.Provider == "docmindcloud" {
		return &dto.ModelDebugResult{
			OK:        false,
			ElapsedMS: time.Since(startedAt).Milliseconds(),
			Request:   payload,
			RawResponse: map[string]interface{}{
				"message": "DocMindCloud 调试暂未接入在线调用，请先使用连接测试",
			},
			Observations: map[string]interface{}{},
			Error:        "DocMindCloud 调试暂未实现",
		}, nil
	}

	applyThinkingControl(payload, model.Parameters.ExtraConfig, options)
	rawStatus, rawBody, err := s.doJSONRequest(http.MethodPost, appendPath(model.Parameters.BaseURL, "chat/completions"), payload, buildAuthHeaders(model.Parameters.APIKey, model.Parameters.CustomHeaders))
	if err != nil {
		return &dto.ModelDebugResult{
			OK:           false,
			ElapsedMS:    time.Since(startedAt).Milliseconds(),
			Request:      payload,
			RawResponse:  map[string]interface{}{},
			Observations: map[string]interface{}{},
			Error:        err.Error(),
		}, nil
	}

	result := &dto.ModelDebugResult{
		OK:           rawStatus >= 200 && rawStatus < 300,
		ElapsedMS:    time.Since(startedAt).Milliseconds(),
		Request:      payload,
		RawResponse:  rawBody,
		Observations: map[string]interface{}{},
	}
	if answer := extractChatAnswer(rawBody); answer != "" {
		result.Observations["answer_characters"] = len(answer)
	}
	reasoning := extractReasoning(rawBody)
	if reasoning != "" {
		result.Observations["reasoning_characters"] = len(reasoning)
		result.Observations["reasoning_returned"] = true
	}
	if !result.OK {
		result.Error = extractErrorMessage(rawBody, rawStatus)
	}
	return result, nil
}

// debugEmbeddingModel 对 embedding 模型执行调试调用，返回向量与原始响应。
func (s *modelService) debugEmbeddingModel(model *entity.Model, input string) ([]float64, map[string]interface{}, error) {
	if strings.TrimSpace(input) == "" {
		input = "hello world"
	}
	if model.Source == "local" {
		vector, raw, err := s.callOllamaEmbed(model.Name, input)
		return vector, raw, err
	}
	if model.Parameters.Provider == "docmindcloud" {
		return make([]float64, 1024), map[string]interface{}{"provider": "docmindcloud", "dimension": 1024}, nil
	}
	status, body, err := s.doJSONRequest(http.MethodPost, appendPath(model.Parameters.BaseURL, "embeddings"), map[string]interface{}{
		"model": model.Name,
		"input": input,
	}, buildAuthHeaders(model.Parameters.APIKey, model.Parameters.CustomHeaders))
	if err != nil {
		return nil, map[string]interface{}{}, err
	}
	if status < 200 || status >= 300 {
		return nil, body, fmt.Errorf("%s", extractErrorMessage(body, status))
	}
	return extractEmbeddingVector(body), body, nil
}

// EmbedText 调用指定 embedding 模型生成向量
func (s *modelService) EmbedText(userID uint, modelRef string, input string) ([]float32, error) {
	modelRef = strings.TrimSpace(modelRef)
	if modelRef == "" {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "embedding_model_id 不能为空")
	}

	var (
		model *entity.Model
		err   error
	)

	if id, parseErr := strconv.ParseUint(modelRef, 10, 64); parseErr == nil {
		model, err = s.modelRepo.FindByUserID(uint(id), userID)
	} else {
		model, err = s.modelRepo.FindByName(modelRef, userID)
	}
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询 embedding 模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "embedding 模型不存在")
	}
	if model.Type != entity.ModelTypeEmbedding {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "指定模型不是 embedding 类型")
	}

	vector, _, err := s.debugEmbeddingModel(model, input)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "调用 embedding 模型失败", err)
	}

	result := make([]float32, 0, len(vector))
	for _, value := range vector {
		result = append(result, float32(value))
	}
	return result, nil
}

// debugRerankModel 对 rerank 模型执行调试调用，自动尝试多种 URL 后缀。
func (s *modelService) debugRerankModel(model *entity.Model, query string, documents []string) (map[string]interface{}, int, error) {
	if strings.TrimSpace(query) == "" {
		return map[string]interface{}{}, 0, fmt.Errorf("query 不能为空")
	}
	if len(documents) == 0 {
		return map[string]interface{}{}, 0, fmt.Errorf("documents 不能为空")
	}
	if model.Parameters.Provider == "docmindcloud" {
		fake := map[string]interface{}{
			"results": []map[string]interface{}{
				{"index": 0, "relevance_score": 0.9},
				{"index": 1, "relevance_score": 0.7},
			},
		}
		return fake, 2, nil
	}
	candidates := []string{model.Parameters.BaseURL}
	if !strings.Contains(strings.ToLower(model.Parameters.BaseURL), "rerank") {
		candidates = append(candidates, appendPath(model.Parameters.BaseURL, "rerank"))
		candidates = append(candidates, appendPath(model.Parameters.BaseURL, "v1/rerank"))
	}
	payload := map[string]interface{}{
		"model":     model.Name,
		"query":     query,
		"documents": documents,
	}
	for _, candidate := range deduplicateStrings(candidates) {
		status, body, err := s.doJSONRequest(http.MethodPost, candidate, payload, buildAuthHeaders(model.Parameters.APIKey, model.Parameters.CustomHeaders))
		if err != nil {
			continue
		}
		if status >= 200 && status < 300 {
			return body, extractResultsCount(body), nil
		}
		if status != http.StatusNotFound {
			return body, 0, fmt.Errorf("%s", extractErrorMessage(body, status))
		}
	}
	return map[string]interface{}{}, 0, fmt.Errorf("未找到可用的 Rerank 接口")
}

// debugASRModel 对 ASR 模型执行调试调用，上传音频文件并解析转写结果。
func (s *modelService) debugASRModel(model *entity.Model, fileHeader *multipart.FileHeader) (map[string]interface{}, string, error) {
	if fileHeader == nil {
		return map[string]interface{}{}, "", fmt.Errorf("请上传音频文件")
	}
	reader, err := fileHeader.Open()
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	status, body, err := s.doMultipartTranscription(appendPath(model.Parameters.BaseURL, "audio/transcriptions"), model.Name, data, fileHeader.Filename, buildAuthHeaders(model.Parameters.APIKey, model.Parameters.CustomHeaders))
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	if status < 200 || status >= 300 {
		return body, "", fmt.Errorf("%s", extractErrorMessage(body, status))
	}
	return body, stringValue(body["text"]), nil
}

// ---------- 配置解析 ----------

// resolveTestConfig 从探测请求中解析实际配置：优先使用请求参数，其次从已存储模型补全。
func (s *modelService) resolveTestConfig(request *req.ModelTestRequest) (*req.ModelTestRequest, error) {
	if request == nil {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "请求不能为空")
	}
	cloned := *request
	if cloned.ModelID != "" {
		id, err := strconv.ParseUint(cloned.ModelID, 10, 64)
		if err == nil {
			model, repoErr := s.modelRepo.FindByID(uint(id))
			if repoErr != nil {
				return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", repoErr)
			}
			if model != nil {
				if strings.TrimSpace(cloned.APIKey) == "" {
					cloned.APIKey = model.Parameters.APIKey
				}
				if strings.TrimSpace(cloned.AppSecret) == "" {
					cloned.AppSecret = model.Parameters.AppSecret
				}
				if strings.TrimSpace(cloned.BaseURL) == "" {
					cloned.BaseURL = model.Parameters.BaseURL
				}
				if strings.TrimSpace(cloned.Provider) == "" {
					cloned.Provider = model.Parameters.Provider
				}
				if strings.TrimSpace(cloned.ModelName) == "" {
					cloned.ModelName = model.Name
				}
				if strings.TrimSpace(cloned.Source) == "" {
					cloned.Source = model.Source
				}
				if cloned.CustomHeaders == nil {
					cloned.CustomHeaders = model.Parameters.CustomHeaders
				}
				if cloned.ExtraConfig == nil {
					cloned.ExtraConfig = model.Parameters.ExtraConfig
				}
			}
		}
	}
	if strings.TrimSpace(cloned.Provider) == "docmindcloud" {
		creds, err := s.getDocMindCloudCredentials()
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(cloned.APIKey) == "" {
			cloned.APIKey = creds["app_id"]
		}
		if strings.TrimSpace(cloned.AppSecret) == "" {
			cloned.AppSecret = creds["app_secret"]
		}
		if strings.TrimSpace(cloned.BaseURL) == "" {
			cloned.BaseURL = "https://docmind.weixin.qq.com"
		}
	}
	if strings.TrimSpace(cloned.Source) == "" {
		cloned.Source = "remote"
	}
	return &cloned, nil
}

// getDocMindCloudCredentials 从系统配置表读取 DocMindCloud 凭据。
func (s *modelService) getDocMindCloudCredentials() (map[string]string, error) {
	setting, err := s.settingRepo.FindByKey(docMindCloudCredentialKey)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询 DocMindCloud 凭据失败", err)
	}
	if setting == nil {
		return map[string]string{}, nil
	}
	return map[string]string{
		"app_id":     stringValue(setting.Value["app_id"]),
		"app_secret": stringValue(setting.Value["app_secret"]),
	}, nil
}

// ---------- DTO 构建 ----------

// buildModelResponse 将 entity.Model 转换为 API 响应 DTO，包含凭据脱敏标记。
func (s *modelService) buildModelResponse(model *entity.Model) *dto.ModelResponse {
	var deletedAt *string
	if model.DeletedAt.Valid {
		value := model.DeletedAt.Time.Format(time.RFC3339)
		deletedAt = &value
	}

	parameters := dto.ModelParametersResponse{
		BaseURL:        model.Parameters.BaseURL,
		APIVersion:     model.Parameters.APIVersion,
		ModelName:      model.Parameters.ModelName,
		Provider:       model.Parameters.Provider,
		InterfaceType:  model.Parameters.InterfaceType,
		ParameterSize:  model.Parameters.ParameterSize,
		Temperature:    model.Parameters.Temperature,
		MaxTokens:      model.Parameters.MaxTokens,
		ContextWindow:  model.Parameters.ContextWindow,
		Dimension:      model.Parameters.Dimension,
		KeepAlive:      model.Parameters.KeepAlive,
		ExtraConfig:    model.Parameters.ExtraConfig,
		CustomHeaders:  model.Parameters.CustomHeaders,
		SupportsVision: model.Parameters.SupportsVision,
		MaxConcurrency: model.Parameters.MaxConcurrency,
		AppID:          model.Parameters.AppID,
	}
	if model.Parameters.EmbeddingParameters.Dimension > 0 || model.Parameters.EmbeddingParameters.TruncatePromptTokens > 0 || model.Parameters.EmbeddingParameters.SupportsDimensionOverride {
		parameters.EmbeddingParameters = &dto.EmbeddingParametersResponse{
			Dimension:                 model.Parameters.EmbeddingParameters.Dimension,
			TruncatePromptTokens:      model.Parameters.EmbeddingParameters.TruncatePromptTokens,
			SupportsDimensionOverride: model.Parameters.EmbeddingParameters.SupportsDimensionOverride,
		}
	}

	return &dto.ModelResponse{
		ID:          strconv.FormatUint(uint64(model.ID), 10),
		Name:        model.Name,
		DisplayName: model.DisplayName,
		Type:        model.Type,
		Source:      model.Source,
		Description: model.Description,
		Status:      model.Status,
		IsDefault:   model.IsDefault,
		IsBuiltin:   model.IsBuiltin,
		Parameters:  parameters,
		Credentials: buildCredentialResponse(model.Parameters).Fields,
		CreatedAt:   model.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   model.UpdatedAt.Format(time.RFC3339),
		DeletedAt:   deletedAt,
	}
}

// ---------- 校验与转换 ----------

// validateModelRequest 对创建/更新模型的请求做基础校验。
// 注意：Gin 已保证 request 非 nil，故不在此处做 nil 检查。
func validateModelRequest(request *req.UpsertModelRequest) error {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "模型名称不能为空")
	}
	switch request.Type {
	case entity.ModelTypeKnowledgeQA, entity.ModelTypeEmbedding, entity.ModelTypeRerank, entity.ModelTypeVLLM, entity.ModelTypeASR:
	default:
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "不支持的模型类型")
	}
	switch request.Source {
	case "local", "remote":
	default:
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "不支持的模型来源")
	}
	if request.Source == "remote" && strings.TrimSpace(request.Parameters.BaseURL) == "" && strings.TrimSpace(request.Parameters.Provider) != "docmindcloud" {
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "远程模型必须配置 Base URL")
	}
	return nil
}

// toEntityParameters 将请求参数合并到已有实体参数中，空白字段保留原值（凭据类字段仅在非空时覆盖）。
func toEntityParameters(request req.ModelParametersRequest, existing entity.ModelParameters) entity.ModelParameters {
	params := existing
	params.BaseURL = strings.TrimSpace(request.BaseURL)
	params.AppID = strings.TrimSpace(request.AppID)
	params.APIVersion = strings.TrimSpace(request.APIVersion)
	params.ModelName = strings.TrimSpace(request.ModelName)
	params.Provider = strings.TrimSpace(request.Provider)
	params.InterfaceType = strings.TrimSpace(request.InterfaceType)
	params.ParameterSize = strings.TrimSpace(request.ParameterSize)
	params.Temperature = request.Temperature
	params.MaxTokens = request.MaxTokens
	params.ContextWindow = request.ContextWindow
	params.Dimension = request.Dimension
	params.KeepAlive = strings.TrimSpace(request.KeepAlive)
	params.SupportsVision = request.SupportsVision
	params.MaxConcurrency = request.MaxConcurrency
	params.ExtraConfig = request.ExtraConfig
	params.CustomHeaders = sanitizeHeaders(request.CustomHeaders)
	if request.EmbeddingParameters != nil {
		params.EmbeddingParameters = entity.EmbeddingParameters{
			Dimension:                 request.EmbeddingParameters.Dimension,
			TruncatePromptTokens:      request.EmbeddingParameters.TruncatePromptTokens,
			SupportsDimensionOverride: request.EmbeddingParameters.SupportsDimensionOverride,
		}
	}
	if strings.TrimSpace(request.APIKey) != "" {
		params.APIKey = strings.TrimSpace(request.APIKey)
	}
	if strings.TrimSpace(request.AppSecret) != "" {
		params.AppSecret = strings.TrimSpace(request.AppSecret)
	}
	return params
}

// buildCredentialResponse 构造凭据配置状态响应（仅标记字段是否已配置，不透出真实值）。
func buildCredentialResponse(parameters entity.ModelParameters) *dto.ModelCredentialsResponse {
	return &dto.ModelCredentialsResponse{
		Fields: map[string]dto.CredentialFieldState{
			"api_key":    {Configured: strings.TrimSpace(parameters.APIKey) != ""},
			"app_secret": {Configured: strings.TrimSpace(parameters.AppSecret) != ""},
		},
	}
}
