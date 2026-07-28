package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	pkgerrors "docmind/pkg/errors"

	"github.com/google/uuid"
)

const docMindCloudCredentialKey = "docmindcloud_credentials"

type ollamaDownloadTask struct {
	ID        string
	ModelName string
	Status    string
	Progress  float64
	Message   string
	StartTime time.Time
	EndTime   *time.Time
}

type modelService struct {
	modelRepo   repository.ModelRepository
	settingRepo repository.SystemSettingRepository
	httpClient  *http.Client

	taskMu sync.RWMutex
	tasks  map[string]*ollamaDownloadTask
}

// NewModelService 创建模型服务
func NewModelService(
	modelRepo repository.ModelRepository,
	settingRepo repository.SystemSettingRepository,
) ModelService {
	return &modelService{
		modelRepo:   modelRepo,
		settingRepo: settingRepo,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		tasks: map[string]*ollamaDownloadTask{},
	}
}

func (s *modelService) CreateModel(request *req.UpsertModelRequest) (*dto.ModelResponse, error) {
	if err := validateModelRequest(request); err != nil {
		return nil, err
	}

	existing, err := s.modelRepo.FindByName(strings.TrimSpace(request.Name))
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if existing != nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceAlreadyExists, "模型名称已存在")
	}

	model := &entity.Model{
		Name:        strings.TrimSpace(request.Name),
		DisplayName: strings.TrimSpace(request.DisplayName),
		Type:        request.Type,
		Source:      request.Source,
		Description: strings.TrimSpace(request.Description),
		Status:      entity.ModelStatusActive,
		IsDefault:   request.IsDefault,
		IsBuiltin:   request.IsBuiltin,
		Parameters:  toEntityParameters(request.Parameters, entity.ModelParameters{}),
	}
	if err := s.modelRepo.Create(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "创建模型失败", err)
	}
	return s.buildModelResponse(model), nil
}

func (s *modelService) ListModels(modelType string) ([]*dto.ModelResponse, error) {
	models, err := s.modelRepo.List(modelType)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型列表失败", err)
	}

	result := make([]*dto.ModelResponse, 0, len(models))
	for _, model := range models {
		result = append(result, s.buildModelResponse(model))
	}
	return result, nil
}

func (s *modelService) GetModel(id uint) (*dto.ModelResponse, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	return s.buildModelResponse(model), nil
}

func (s *modelService) UpdateModel(id uint, request *req.UpsertModelRequest) (*dto.ModelResponse, error) {
	if err := validateModelRequest(request); err != nil {
		return nil, err
	}

	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}

	if strings.TrimSpace(request.Name) != model.Name {
		existing, err := s.modelRepo.FindByName(strings.TrimSpace(request.Name))
		if err != nil {
			return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
		}
		if existing != nil && existing.ID != model.ID {
			return nil, pkgerrors.New(pkgerrors.CodeResourceAlreadyExists, "模型名称已存在")
		}
	}

	model.Name = strings.TrimSpace(request.Name)
	model.DisplayName = strings.TrimSpace(request.DisplayName)
	model.Type = request.Type
	model.Source = request.Source
	model.Description = strings.TrimSpace(request.Description)
	model.IsDefault = request.IsDefault
	model.IsBuiltin = request.IsBuiltin
	model.Parameters = toEntityParameters(request.Parameters, model.Parameters)

	if err := s.modelRepo.Update(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "更新模型失败", err)
	}
	return s.buildModelResponse(model), nil
}

func (s *modelService) DeleteModel(id uint) error {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}
	if model.IsBuiltin {
		return pkgerrors.New(pkgerrors.CodeForbidden, "内置模型不允许删除")
	}
	if err := s.modelRepo.Delete(id); err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "删除模型失败", err)
	}
	return nil
}

func (s *modelService) PutModelCredentials(id uint, request *req.PutModelCredentialsRequest) (*dto.ModelCredentialsResponse, error) {
	model, err := s.modelRepo.FindByID(id)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "查询模型失败", err)
	}
	if model == nil {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "模型不存在")
	}

	if request.APIKey != nil {
		model.Parameters.APIKey = strings.TrimSpace(*request.APIKey)
	}
	if request.AppSecret != nil {
		model.Parameters.AppSecret = strings.TrimSpace(*request.AppSecret)
	}
	if err := s.modelRepo.Update(model); err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "更新模型凭据失败", err)
	}
	return buildCredentialResponse(model.Parameters), nil
}

func (s *modelService) DeleteModelCredentialField(id uint, field string) (*dto.ModelCredentialsResponse, error) {
	model, err := s.modelRepo.FindByID(id)
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
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "删除模型凭据失败", err)
	}
	return buildCredentialResponse(model.Parameters), nil
}

func (s *modelService) ListProviders(modelType string) []*dto.ModelProviderOptionResponse {
	all := []*dto.ModelProviderOptionResponse{
		{Value: "openai", Label: "OpenAI", Description: "OpenAI 官方与兼容接口", DefaultURLs: map[string]string{"chat": "https://api.openai.com/v1", "embedding": "https://api.openai.com/v1", "rerank": "https://api.openai.com/v1", "vllm": "https://api.openai.com/v1", "asr": "https://api.openai.com/v1"}, ModelTypes: []string{"chat", "embedding", "rerank", "vllm", "asr"}},
		{Value: "azure_openai", Label: "Azure OpenAI", Description: "Azure OpenAI 服务", DefaultURLs: map[string]string{"chat": "https://{resource}.openai.azure.com", "embedding": "https://{resource}.openai.azure.com", "vllm": "https://{resource}.openai.azure.com", "asr": "https://{resource}.openai.azure.com"}, ModelTypes: []string{"chat", "embedding", "vllm", "asr"}},
		{Value: "aliyun", Label: "阿里云 DashScope", Description: "阿里云百炼 / DashScope", DefaultURLs: map[string]string{"chat": "https://dashscope.aliyuncs.com/compatible-mode/v1", "embedding": "https://dashscope.aliyuncs.com/compatible-mode/v1", "rerank": "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank", "vllm": "https://dashscope.aliyuncs.com/compatible-mode/v1"}, ModelTypes: []string{"chat", "embedding", "rerank", "vllm"}},
		{Value: "zhipu", Label: "智谱 GLM", Description: "智谱开放平台", DefaultURLs: map[string]string{"chat": "https://open.bigmodel.cn/api/paas/v4", "embedding": "https://open.bigmodel.cn/api/paas/v4/embeddings", "vllm": "https://open.bigmodel.cn/api/paas/v4"}, ModelTypes: []string{"chat", "embedding", "vllm"}},
		{Value: "siliconflow", Label: "SiliconFlow", Description: "SiliconFlow API", DefaultURLs: map[string]string{"chat": "https://api.siliconflow.cn/v1", "embedding": "https://api.siliconflow.cn/v1", "rerank": "https://api.siliconflow.cn/v1"}, ModelTypes: []string{"chat", "embedding", "rerank"}},
		{Value: "jina", Label: "Jina AI", Description: "Jina Embedding / Rerank", DefaultURLs: map[string]string{"embedding": "https://api.jina.ai/v1", "rerank": "https://api.jina.ai/v1"}, ModelTypes: []string{"embedding", "rerank"}},
		{Value: "volcengine", Label: "火山引擎", Description: "火山引擎 Ark", DefaultURLs: map[string]string{"chat": "https://ark.cn-beijing.volces.com/api/v3", "embedding": "https://ark.cn-beijing.volces.com/api/v3", "rerank": "https://ark.cn-beijing.volces.com/api/v3"}, ModelTypes: []string{"chat", "embedding", "rerank"}},
		{Value: "gemini", Label: "Gemini", Description: "Google Gemini", DefaultURLs: map[string]string{"chat": "https://generativelanguage.googleapis.com/v1beta/openai", "embedding": "https://generativelanguage.googleapis.com/v1beta"}, ModelTypes: []string{"chat", "embedding"}},
		{Value: "openrouter", Label: "OpenRouter", Description: "OpenRouter 聚合接口", DefaultURLs: map[string]string{"chat": "https://openrouter.ai/api/v1", "embedding": "https://openrouter.ai/api/v1"}, ModelTypes: []string{"chat", "embedding"}},
		{Value: "nvidia", Label: "NVIDIA", Description: "NVIDIA API", DefaultURLs: map[string]string{"chat": "https://integrate.api.nvidia.com/v1", "embedding": "https://integrate.api.nvidia.com/v1", "rerank": "https://ai.api.nvidia.com/v1/retrieval/nvidia/reranking", "vllm": "https://integrate.api.nvidia.com/v1"}, ModelTypes: []string{"chat", "embedding", "rerank", "vllm"}},
		{Value: "novita", Label: "Novita", Description: "Novita AI", DefaultURLs: map[string]string{"chat": "https://api.novita.ai/openai/v1", "embedding": "https://api.novita.ai/openai/v1", "vllm": "https://api.novita.ai/openai/v1"}, ModelTypes: []string{"chat", "embedding", "vllm"}},
		{Value: "lkeap", Label: "腾讯云 LKEAP", Description: "腾讯云 LKEAP Rerank", DefaultURLs: map[string]string{"rerank": "https://lkeap.tencentcloudapi.com"}, ModelTypes: []string{"rerank"}},
		{Value: "docmindcloud", Label: "DocMind Cloud", Description: "DocMind Cloud 原子能力", DefaultURLs: map[string]string{"chat": "https://docmind.weixin.qq.com", "embedding": "https://docmind.weixin.qq.com", "rerank": "https://docmind.weixin.qq.com", "vllm": "https://docmind.weixin.qq.com"}, ModelTypes: []string{"chat", "embedding", "rerank", "vllm"}},
		{Value: "generic", Label: "自定义", Description: "自定义 OpenAI 兼容接口", DefaultURLs: map[string]string{}, ModelTypes: []string{"chat", "embedding", "rerank", "vllm", "asr"}},
	}

	if modelType == "" {
		return all
	}

	filtered := make([]*dto.ModelProviderOptionResponse, 0, len(all))
	for _, item := range all {
		for _, supportedType := range item.ModelTypes {
			if supportedType == modelType {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func (s *modelService) SaveDocMindCloudCredentials(appID, appSecret string) error {
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	if appID == "" || appSecret == "" {
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "APPID 和 APPSECRET 不能为空")
	}

	setting := &entity.SystemSetting{
		Key: docMindCloudCredentialKey,
		Value: entity.JSONMap{
			"app_id":     appID,
			"app_secret": appSecret,
			"updated_at": time.Now().Format(time.RFC3339),
		},
	}
	if err := s.settingRepo.Upsert(setting); err != nil {
		return pkgerrors.NewWithErr(pkgerrors.CodeInternalError, "保存 DocMindCloud 凭据失败", err)
	}
	return nil
}

func (s *modelService) GetDocMindCloudStatus() (*dto.DocMindCloudStatusResponse, error) {
	creds, err := s.getDocMindCloudCredentials()
	if err != nil {
		return nil, err
	}
	hasCreds := creds["app_id"] != "" && creds["app_secret"] != ""
	return &dto.DocMindCloudStatusResponse{
		HasModels:   hasCreds,
		NeedsReinit: false,
	}, nil
}

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
	if cfg.Provider == "docmindcloud" {
		return map[string]interface{}{"available": true, "message": "DocMindCloud 凭据已配置"}, nil
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

func (s *modelService) GetOllamaStatus() (*dto.OllamaStatusResponse, error) {
	url := appendPath(s.ollamaBaseURL(), "version")
	status, body, err := s.doJSONRequest(http.MethodGet, url, nil, nil)
	if err != nil {
		return &dto.OllamaStatusResponse{
			Available: false,
			Error:     err.Error(),
			BaseURL:   s.ollamaBaseURL(),
		}, nil
	}
	if status >= 200 && status < 300 {
		version, _ := body["version"].(string)
		return &dto.OllamaStatusResponse{
			Available: true,
			Version:   version,
			BaseURL:   s.ollamaBaseURL(),
		}, nil
	}
	return &dto.OllamaStatusResponse{
		Available: false,
		Error:     extractErrorMessage(body, status),
		BaseURL:   s.ollamaBaseURL(),
	}, nil
}

func (s *modelService) ListOllamaModels() ([]*dto.OllamaModelInfoResponse, error) {
	status, body, err := s.doJSONRequest(http.MethodGet, appendPath(s.ollamaBaseURL(), "tags"), nil, nil)
	if err != nil {
		return nil, pkgerrors.NewWithErr(pkgerrors.CodeServiceUnavailable, "获取 Ollama 模型列表失败", err)
	}
	if status < 200 || status >= 300 {
		return nil, pkgerrors.New(pkgerrors.CodeServiceUnavailable, extractErrorMessage(body, status))
	}

	modelsRaw, _ := body["models"].([]interface{})
	result := make([]*dto.OllamaModelInfoResponse, 0, len(modelsRaw))
	for _, item := range modelsRaw {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, &dto.OllamaModelInfoResponse{
			Name:       stringValue(itemMap["name"]),
			Size:       int64Value(itemMap["size"]),
			Digest:     stringValue(itemMap["digest"]),
			ModifiedAt: stringValue(itemMap["modified_at"]),
		})
	}
	return result, nil
}

func (s *modelService) CheckOllamaModels(models []string) (map[string]bool, error) {
	list, err := s.ListOllamaModels()
	if err != nil {
		return nil, err
	}

	installed := map[string]bool{}
	for _, model := range list {
		installed[model.Name] = true
	}

	result := map[string]bool{}
	for _, name := range models {
		result[name] = installed[name]
	}
	return result, nil
}

func (s *modelService) DownloadOllamaModel(modelName string) (*dto.DownloadTaskResponse, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, pkgerrors.New(pkgerrors.CodeInvalidParam, "模型名称不能为空")
	}

	task := &ollamaDownloadTask{
		ID:        uuid.NewString(),
		ModelName: modelName,
		Status:    "pending",
		Progress:  0,
		Message:   "下载任务已创建",
		StartTime: time.Now(),
	}

	s.taskMu.Lock()
	s.tasks[task.ID] = task
	s.taskMu.Unlock()

	go s.runOllamaDownload(task)
	return s.toDownloadTaskResponse(task), nil
}

func (s *modelService) GetOllamaDownloadProgress(taskID string) (*dto.DownloadTaskResponse, error) {
	s.taskMu.RLock()
	task, ok := s.tasks[taskID]
	s.taskMu.RUnlock()
	if !ok {
		return nil, pkgerrors.New(pkgerrors.CodeResourceNotFound, "下载任务不存在")
	}
	return s.toDownloadTaskResponse(task), nil
}

func (s *modelService) ListOllamaDownloadTasks() ([]*dto.DownloadTaskResponse, error) {
	s.taskMu.RLock()
	tasks := make([]*ollamaDownloadTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	s.taskMu.RUnlock()

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartTime.After(tasks[j].StartTime)
	})

	result := make([]*dto.DownloadTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, s.toDownloadTaskResponse(task))
	}
	return result, nil
}

func (s *modelService) DebugModel(id uint, input string, documents []string, options map[string]interface{}, fileHeader *multipart.FileHeader) (*dto.ModelDebugResult, error) {
	model, err := s.modelRepo.FindByID(id)
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
		return nil, body, fmt.Errorf(extractErrorMessage(body, status))
	}
	return extractEmbeddingVector(body), body, nil
}

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
			return body, 0, fmt.Errorf(extractErrorMessage(body, status))
		}
	}
	return map[string]interface{}{}, 0, fmt.Errorf("未找到可用的 Rerank 接口")
}

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
		return body, "", fmt.Errorf(extractErrorMessage(body, status))
	}
	return body, stringValue(body["text"]), nil
}

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

func (s *modelService) runOllamaDownload(task *ollamaDownloadTask) {
	s.updateTask(task.ID, func(t *ollamaDownloadTask) {
		t.Status = "downloading"
		t.Message = "开始下载"
	})

	payload := map[string]interface{}{
		"model":  task.ModelName,
		"stream": true,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, appendPath(s.ollamaBaseURL(), "pull"), bytes.NewReader(raw))
	if err != nil {
		s.failTask(task.ID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.failTask(task.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		s.failTask(task.ID, fmt.Errorf("下载失败: %s", strings.TrimSpace(string(body))))
		return
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			s.failTask(task.ID, err)
			return
		}

		statusText := stringValue(event["status"])
		total := float64Value(event["total"])
		completed := float64Value(event["completed"])
		progress := 0.0
		if total > 0 {
			progress = (completed / total) * 100
		}

		s.updateTask(task.ID, func(t *ollamaDownloadTask) {
			if progress > t.Progress {
				t.Progress = progress
			}
			if statusText != "" {
				t.Message = statusText
			}
		})

		if statusText == "success" {
			now := time.Now()
			s.updateTask(task.ID, func(t *ollamaDownloadTask) {
				t.Status = "completed"
				t.Progress = 100
				t.Message = "下载完成"
				t.EndTime = &now
			})
			return
		}
		if errorText := stringValue(event["error"]); errorText != "" {
			s.failTask(task.ID, fmt.Errorf(errorText))
			return
		}
	}

	now := time.Now()
	s.updateTask(task.ID, func(t *ollamaDownloadTask) {
		t.Status = "completed"
		t.Progress = 100
		t.Message = "下载完成"
		t.EndTime = &now
	})
}

func (s *modelService) updateTask(taskID string, fn func(task *ollamaDownloadTask)) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	if task, ok := s.tasks[taskID]; ok {
		fn(task)
	}
}

func (s *modelService) failTask(taskID string, err error) {
	now := time.Now()
	s.updateTask(taskID, func(task *ollamaDownloadTask) {
		task.Status = "failed"
		task.Message = err.Error()
		task.EndTime = &now
	})
}

func (s *modelService) toDownloadTaskResponse(task *ollamaDownloadTask) *dto.DownloadTaskResponse {
	resp := &dto.DownloadTaskResponse{
		ID:        task.ID,
		ModelName: task.ModelName,
		Status:    task.Status,
		Progress:  task.Progress,
		Message:   task.Message,
		StartTime: task.StartTime.Format(time.RFC3339),
	}
	if task.EndTime != nil {
		resp.EndTime = task.EndTime.Format(time.RFC3339)
	}
	return resp
}

func (s *modelService) ollamaBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("OLLAMA_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return "http://localhost:11434/api"
}

func (s *modelService) doJSONRequest(method, targetURL string, payload interface{}, headers http.Header) (int, map[string]interface{}, error) {
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}

	request, err := http.NewRequestWithContext(context.Background(), method, targetURL, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := s.httpClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	if len(rawBody) == 0 {
		return response.StatusCode, map[string]interface{}{}, nil
	}

	result := map[string]interface{}{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		result["raw"] = string(rawBody)
	}
	return response.StatusCode, result, nil
}

func (s *modelService) doMultipartTranscription(targetURL, modelName string, fileData []byte, fileName string, headers http.Header) (int, map[string]interface{}, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	_ = writer.WriteField("model", modelName)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return 0, nil, err
	}
	if _, err := part.Write(fileData); err != nil {
		return 0, nil, err
	}
	if err := writer.Close(); err != nil {
		return 0, nil, err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, targetURL, buffer)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}

	response, err := s.httpClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	result := map[string]interface{}{}
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &result); err != nil {
			result["raw"] = string(rawBody)
		}
	}
	return response.StatusCode, result, nil
}

func (s *modelService) callOllamaEmbed(modelName, input string) ([]float64, map[string]interface{}, error) {
	payload := map[string]interface{}{
		"model": modelName,
		"input": input,
	}
	status, body, err := s.doJSONRequest(http.MethodPost, appendPath(s.ollamaBaseURL(), "embed"), payload, nil)
	if err != nil {
		return nil, map[string]interface{}{}, err
	}
	if status < 200 || status >= 300 {
		return nil, body, fmt.Errorf(extractErrorMessage(body, status))
	}
	return extractEmbeddingVector(body), body, nil
}

func (s *modelService) callOllamaChat(modelName, input string, options map[string]interface{}, fileHeader *multipart.FileHeader, withVision bool) (map[string]interface{}, string, error) {
	message := map[string]interface{}{
		"role":    "user",
		"content": input,
	}
	if withVision && fileHeader != nil {
		data, err := fileHeaderBytes(fileHeader)
		if err != nil {
			return map[string]interface{}{}, "", err
		}
		message["images"] = []string{base64.StdEncoding.EncodeToString(data)}
	}

	payload := map[string]interface{}{
		"model":    modelName,
		"messages": []map[string]interface{}{message},
		"stream":   false,
	}
	ollamaOptions := map[string]interface{}{}
	if temperature, ok := options["temperature"]; ok {
		ollamaOptions["temperature"] = temperature
	}
	if topP, ok := options["top_p"]; ok {
		ollamaOptions["top_p"] = topP
	}
	if maxTokens, ok := options["max_tokens"]; ok {
		ollamaOptions["num_predict"] = maxTokens
	}
	if len(ollamaOptions) > 0 {
		payload["options"] = ollamaOptions
	}

	status, body, err := s.doJSONRequest(http.MethodPost, appendPath(s.ollamaBaseURL(), "chat"), payload, nil)
	if err != nil {
		return map[string]interface{}{}, "", err
	}
	if status < 200 || status >= 300 {
		return body, "", fmt.Errorf(extractErrorMessage(body, status))
	}
	answer := ""
	if msg, ok := body["message"].(map[string]interface{}); ok {
		answer = stringValue(msg["content"])
	}
	return body, answer, nil
}

func validateModelRequest(request *req.UpsertModelRequest) error {
	if request == nil {
		return pkgerrors.New(pkgerrors.CodeInvalidParam, "请求不能为空")
	}
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

func buildCredentialResponse(parameters entity.ModelParameters) *dto.ModelCredentialsResponse {
	return &dto.ModelCredentialsResponse{
		Fields: map[string]dto.CredentialFieldState{
			"api_key":    {Configured: strings.TrimSpace(parameters.APIKey) != ""},
			"app_secret": {Configured: strings.TrimSpace(parameters.AppSecret) != ""},
		},
	}
}

func buildAuthHeaders(apiKey string, customHeaders map[string]string) http.Header {
	headers := http.Header{}
	if strings.TrimSpace(apiKey) != "" {
		headers.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	for key, value := range sanitizeHeaders(customHeaders) {
		headers.Set(key, value)
	}
	return headers
}

func sanitizeHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := map[string]string{}
	for key, value := range headers {
		trimmedKey := textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(key))
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		if strings.EqualFold(trimmedKey, "Authorization") || strings.EqualFold(trimmedKey, "Content-Type") {
			continue
		}
		result[trimmedKey] = trimmedValue
	}
	return result
}

func appendPath(baseURL, suffix string) string {
	if strings.TrimSpace(baseURL) == "" {
		return ""
	}
	baseURL = strings.TrimRight(baseURL, "/")
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return baseURL
	}
	if strings.HasPrefix(strings.ToLower(baseURL), "http://") || strings.HasPrefix(strings.ToLower(baseURL), "https://") {
		if strings.HasSuffix(baseURL, "/"+strings.TrimLeft(suffix, "/")) {
			return baseURL
		}
	}
	if strings.Contains(strings.ToLower(baseURL), "/"+strings.ToLower(strings.TrimLeft(suffix, "/"))) {
		return baseURL
	}
	return baseURL + "/" + strings.TrimLeft(suffix, "/")
}

func extractErrorMessage(body map[string]interface{}, status int) string {
	if body == nil {
		return fmt.Sprintf("请求失败，状态码 %d", status)
	}
	if errValue, ok := body["error"]; ok {
		switch typed := errValue.(type) {
		case string:
			if typed != "" {
				return typed
			}
		case map[string]interface{}:
			if msg := stringValue(typed["message"]); msg != "" {
				return msg
			}
			if msg := stringValue(typed["code"]); msg != "" {
				return msg
			}
		}
	}
	if msg := stringValue(body["message"]); msg != "" {
		return msg
	}
	if raw := stringValue(body["raw"]); raw != "" {
		return raw
	}
	return fmt.Sprintf("请求失败，状态码 %d", status)
}

func extractEmbeddingDimension(body map[string]interface{}) int {
	vector := extractEmbeddingVector(body)
	return len(vector)
}

func extractEmbeddingVector(body map[string]interface{}) []float64 {
	dataArray, ok := body["data"].([]interface{})
	if ok && len(dataArray) > 0 {
		if itemMap, ok := dataArray[0].(map[string]interface{}); ok {
			if embArray, ok := itemMap["embedding"].([]interface{}); ok {
				result := make([]float64, 0, len(embArray))
				for _, value := range embArray {
					result = append(result, float64Value(value))
				}
				return result
			}
		}
	}
	embeddingsArray, ok := body["embeddings"].([]interface{})
	if ok && len(embeddingsArray) > 0 {
		if first, ok := embeddingsArray[0].([]interface{}); ok {
			result := make([]float64, 0, len(first))
			for _, value := range first {
				result = append(result, float64Value(value))
			}
			return result
		}
	}
	return nil
}

func extractChatAnswer(body map[string]interface{}) string {
	choices, ok := body["choices"].([]interface{})
	if ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := first["message"].(map[string]interface{}); ok {
				return stringValue(message["content"])
			}
		}
	}
	if message, ok := body["message"].(map[string]interface{}); ok {
		return stringValue(message["content"])
	}
	return ""
}

func extractReasoning(body map[string]interface{}) string {
	choices, ok := body["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]interface{})
	if !ok {
		return ""
	}
	message, ok := first["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	return stringValue(message["reasoning_content"])
}

func extractResultsCount(body map[string]interface{}) int {
	if results, ok := body["results"].([]interface{}); ok {
		return len(results)
	}
	if data, ok := body["data"].([]interface{}); ok {
		return len(data)
	}
	return 0
}

func applyThinkingControl(payload map[string]interface{}, extraConfig map[string]string, options map[string]interface{}) {
	thinkingRaw, exists := options["thinking"]
	if !exists {
		return
	}
	thinking, _ := thinkingRaw.(bool)
	switch strings.TrimSpace(extraConfig["thinking_control"]) {
	case "enable_thinking":
		payload["enable_thinking"] = thinking
	case "thinking_type":
		if thinking {
			payload["thinking_type"] = "enabled"
		} else {
			payload["thinking_type"] = "disabled"
		}
	case "chat_template_kwargs":
		payload["chat_template_kwargs"] = map[string]interface{}{
			"thinking": thinking,
		}
	}
}

func buildTestWAV() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x24, 0x08, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45,
		0x66, 0x6d, 0x74, 0x20, 0x10, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00,
		0x40, 0x1f, 0x00, 0x00, 0x80, 0x3e, 0x00, 0x00, 0x02, 0x00, 0x10, 0x00,
		0x64, 0x61, 0x74, 0x61, 0x00, 0x08, 0x00, 0x00,
	}
}

func fileHeaderToDataURL(header *multipart.FileHeader) (string, error) {
	data, err := fileHeaderBytes(header)
	if err != nil {
		return "", err
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func fileHeaderBytes(header *multipart.FileHeader) ([]byte, error) {
	if header == nil {
		return nil, fmt.Errorf("文件不能为空")
	}
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func fileName(header *multipart.FileHeader) string {
	if header == nil {
		return ""
	}
	return path.Base(header.Filename)
}

func deduplicateStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func int64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

func float64Value(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	default:
		return 0
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func boolMessage(ok bool, successMsg, failMsg string) string {
	if ok {
		return successMsg
	}
	return failMsg
}
