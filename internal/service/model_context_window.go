package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"docmind/internal/model/entity"
)

// 模型上下文大小（context window）获取与消费：
// 1. 写入侧（模型创建/更新时）：支持元数据接口的厂商（Ollama / DashScope / OpenAI 兼容端点）
//    自动拉取并写入 ModelParameters.ContextWindow，不提供接口的厂商走内置映射表兜底；
// 2. 消费侧（对话请求中）：只读数据库缓存 + 离线映射表，不发网络请求，
//    未命中时由调用方回退默认值（memory.DefaultMaxContextTokens）。

// metaHTTPClient 元数据拉取专用客户端：短超时避免拖慢模型创建/更新流程。
var metaHTTPClient = &http.Client{Timeout: 5 * time.Second}

// builtinContextWindows 内置模型上下文映射表（key 为规范化后的模型名）。
// 数据来源：各厂商官方文档核对（OpenAI / DeepSeek / 智谱 / DashScope / Moonshot），
// 仅覆盖不提供元数据接口或接口不可用的常用模型，可随版本更新扩充。
var builtinContextWindows = map[string]int{
	// OpenAI（官方 /v1/models 不返回上下文长度）
	"gpt-4o": 128000, "gpt-4o-mini": 128000,
	"gpt-4-turbo": 128000,
	"gpt-4":       8192, "gpt-3.5-turbo": 16385,
	"o1": 200000, "o1-mini": 128000, "o1-preview": 128000,
	"o3": 200000, "o3-mini": 200000,
	"gpt-4.1": 1047576, "gpt-4.1-mini": 1047576, "gpt-4.1-nano": 1047576,
	// DeepSeek
	"deepseek-chat":     131072,
	"deepseek-reasoner": 131072,
	"deepseek-v3":       131072,
	"deepseek-r1":       131072,
	// DeepSeek V4 系列（官方文档：上下文 1M，输出最大 384K）
	"deepseek-v4":       1000000,
	"deepseek-v4-pro":   1000000,
	"deepseek-v4-flash": 1000000,
	// 小米 MiMo（官方文档：mimo-v2.5 上下文窗口 1M，最大输出 128K）
	"mimo-v2.5":     1000000,
	"mimo-v2.5-pro": 1000000,
	// 智谱 GLM
	"glm-4":       131072,
	"glm-4-plus":  131072,
	"glm-4-air":   131072,
	"glm-4-flash": 131072,
	"glm-4-long":  1000000,
	"glm-4v":      8192,
	"glm-4v-plus": 8192,
	"glm-4.5":     131072,
	"glm-4.5-air": 131072,
	"glm-z1":      131072,
	"glm-z1-air":  131072,
	// 阿里云 DashScope（原生接口不可用时的兜底）
	"qwen-turbo":    1000000,
	"qwen-plus":     131072,
	"qwen-max":      131072,
	"qwen-long":     10000000,
	"qwen-vl-max":   32768,
	"qwen3":         131072,
	"qwen3-vl-plus": 131072,
	// Moonshot Kimi
	"moonshot-v1-8k":   8192,
	"moonshot-v1-32k":  32768,
	"moonshot-v1-128k": 131072,
	"kimi-k2":          131072,
	"kimi-latest":      131072,
}

// openAICompatModelFields OpenAI 兼容 /models 接口中常见的上下文长度字段（按优先级取值）
var openAICompatModelFields = []string{"context_window", "max_context_length", "max_model_len", "context_length"}

// normalizeModelName 规范化模型名：小写、去空白，兼容 org/model 路径与 :tag 后缀。
func normalizeModelName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, ":"); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// lookupBuiltinContextWindow 从内置映射表查询模型上下文大小，未命中返回 0。
// 先精确匹配；未命中时按最长前缀回退（如 gpt-4o-2024-11-20 → gpt-4o），
// 近似值可能导致个别变体误差，仅作为无元数据接口时的兜底。
func lookupBuiltinContextWindow(modelName string) int {
	normalized := normalizeModelName(modelName)
	if normalized == "" {
		return 0
	}
	if w, ok := builtinContextWindows[normalized]; ok {
		return w
	}
	matchedPrefix := ""
	for key := range builtinContextWindows {
		if strings.HasPrefix(normalized, key) && len(key) > len(matchedPrefix) {
			matchedPrefix = key
		}
	}
	if matchedPrefix != "" {
		return builtinContextWindows[matchedPrefix]
	}
	return 0
}

// toInt 将 JSON 数值/字符串安全转为 int。
func toInt(value interface{}) int {
	switch n := value.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i
		}
	}
	return 0
}

// parseOpenAICompatModels 解析 OpenAI 兼容 /v1/models 响应（Moonshot/Groq/xAI/Mistral/vLLM/OpenRouter 等），
// 按 id 匹配模型并从常见字段中提取上下文长度。
func parseOpenAICompatModels(body []byte, modelName string) (int, error) {
	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	normalized := normalizeModelName(modelName)
	for _, item := range payload.Data {
		id, _ := item["id"].(string)
		if normalizeModelName(id) != normalized {
			continue
		}
		for _, field := range openAICompatModelFields {
			if w := toInt(item[field]); w > 0 {
				return w, nil
			}
		}
		return 0, fmt.Errorf("模型 %s 的元数据中未包含上下文长度字段", id)
	}
	return 0, fmt.Errorf("模型列表接口未找到模型 %s", modelName)
}

// parseDashScopeModels 解析 DashScope 原生模型列表响应，
// 匹配 model_name 并提取 capacity.context_window。
func parseDashScopeModels(body []byte, modelName string) (int, error) {
	var payload struct {
		Model struct {
			ProvidedModels []struct {
				ModelName string `json:"model_name"`
				Capacity  struct {
					ContextWindow interface{} `json:"context_window"`
				} `json:"capacity"`
			} `json:"provided_models"`
		} `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	normalized := normalizeModelName(modelName)
	for _, m := range payload.Model.ProvidedModels {
		if normalizeModelName(m.ModelName) != normalized {
			continue
		}
		if w := toInt(m.Capacity.ContextWindow); w > 0 {
			return w, nil
		}
	}
	return 0, fmt.Errorf("DashScope 模型列表未找到模型 %s", modelName)
}

// parseOllamaShow 解析 Ollama /api/show 响应，
// 在 model_info 中查找任意 ".context_length" 结尾的参数（llama/gemma/qwen 等系列）。
func parseOllamaShow(body []byte) (int, error) {
	var payload struct {
		ModelInfo map[string]interface{} `json:"model_info"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	for key, value := range payload.ModelInfo {
		if strings.HasSuffix(key, ".context_length") {
			if w := toInt(value); w > 0 {
				return w, nil
			}
		}
	}
	return 0, fmt.Errorf("Ollama 模型信息中未包含 context_length 参数")
}

// doMetaRequest 发送元数据查询请求并返回状态码与原始响应体。
func doMetaRequest(method, targetURL string, payload interface{}, headers http.Header) (int, []byte, error) {
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
	response, err := metaHTTPClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	return response.StatusCode, body, nil
}

// fetchContextWindow 按模型来源/厂商分发拉取上下文大小。
// 支持：Ollama /api/show、DashScope 原生模型列表、通用 OpenAI 兼容 /models；
// 不支持的厂商返回错误（由调用方回退内置映射表）。
func (s *modelService) fetchContextWindow(model *entity.Model) (int, error) {
	if model == nil {
		return 0, fmt.Errorf("模型为空")
	}
	if model.Parameters.Provider == "docmindcloud" {
		return 0, fmt.Errorf("DocMindCloud 不支持元数据查询")
	}
	if model.Source == "local" {
		return s.fetchOllamaContextWindow(model.Name)
	}
	if model.Parameters.Provider == "aliyun" {
		w, err := s.fetchDashScopeContextWindow(model.Parameters.APIKey, model.Name)
		if err == nil && w > 0 {
			return w, nil
		}
		// 原生接口不可用时回退通用兼容接口（compatible-mode）
	}
	return s.fetchOpenAICompatContextWindow(model)
}

// fetchOllamaContextWindow 通过 Ollama /api/show 拉取本地模型上下文大小。
func (s *modelService) fetchOllamaContextWindow(modelName string) (int, error) {
	status, body, err := doMetaRequest(http.MethodPost, s.ollamaBaseURL()+"/api/show",
		map[string]interface{}{"name": modelName}, nil)
	if err != nil {
		return 0, err
	}
	if status < 200 || status >= 300 {
		return 0, fmt.Errorf("Ollama /api/show 返回状态码 %d", status)
	}
	return parseOllamaShow(body)
}

// fetchDashScopeContextWindow 通过 DashScope 原生模型列表接口拉取上下文大小。
func (s *modelService) fetchDashScopeContextWindow(apiKey, modelName string) (int, error) {
	if strings.TrimSpace(apiKey) == "" {
		return 0, fmt.Errorf("缺少 DashScope API Key")
	}
	status, body, err := doMetaRequest(http.MethodGet, "https://dashscope.aliyuncs.com/api/v1/models",
		nil, buildAuthHeaders(apiKey, nil))
	if err != nil {
		return 0, err
	}
	if status < 200 || status >= 300 {
		return 0, fmt.Errorf("DashScope 模型列表接口返回状态码 %d", status)
	}
	return parseDashScopeModels(body, modelName)
}

// fetchOpenAICompatContextWindow 通过通用 OpenAI 兼容 /models 接口拉取上下文大小。
func (s *modelService) fetchOpenAICompatContextWindow(model *entity.Model) (int, error) {
	target := appendPath(model.Parameters.BaseURL, "models")
	if target == "" {
		return 0, fmt.Errorf("缺少 Base URL")
	}
	status, body, err := doMetaRequest(http.MethodGet, target, nil,
		buildAuthHeaders(model.Parameters.APIKey, model.Parameters.CustomHeaders))
	if err != nil {
		return 0, err
	}
	if status < 200 || status >= 300 {
		return 0, fmt.Errorf("模型列表接口返回状态码 %d", status)
	}
	return parseOpenAICompatModels(body, model.Name)
}

// resolveContextWindow 解析模型上下文大小：远程元数据接口优先，内置映射表兜底。
// 返回 (上下文大小, 失败原因)；成功时原因为空串，失败时原因供缺失记录使用。
func (s *modelService) resolveContextWindow(model *entity.Model) (int, string) {
	fetchErr := ""
	if w, err := s.fetchContextWindow(model); err == nil && w > 0 {
		return w, ""
	} else if err != nil {
		fetchErr = err.Error()
	}
	if w := lookupBuiltinContextWindow(model.Name); w > 0 {
		return w, ""
	}
	if fetchErr != "" {
		return 0, "厂商接口未返回上下文元数据: " + fetchErr
	}
	return 0, "厂商接口未提供上下文元数据且内置映射表未命中"
}

// resolveContextWindowForModel 对话侧解析模型上下文大小（仅读数据库缓存 + 离线映射表，
// 不发网络请求，避免拖慢对话链路）：模型配置 ContextWindow > 内置映射表 > 0。
func (s *chatService) resolveContextWindowForModel(modelID string, userID uint) int {
	model, err := s.resolveModelEntity(modelID, userID)
	if err != nil || model == nil {
		return 0
	}
	if model.Parameters.ContextWindow > 0 {
		return model.Parameters.ContextWindow
	}
	return lookupBuiltinContextWindow(model.Name)
}

// resolveModelEntity 按 modelID 解析模型实体，语义与 pipeline.createChatModel 保持一致：
// "default"/空 时优先当前用户 KnowledgeQA 默认模型，回退系统级默认（user_id=0）；否则按数字 ID 查找。
func (s *chatService) resolveModelEntity(modelID string, userID uint) (*entity.Model, error) {
	modelRepo := s.modelFactory.ModelRepo()
	if modelID == "" || modelID == "default" {
		models, err := modelRepo.List(entity.ModelTypeKnowledgeQA, userID)
		if err != nil {
			return nil, err
		}
		if len(models) == 0 {
			models, err = modelRepo.List(entity.ModelTypeKnowledgeQA, 0)
			if err != nil {
				return nil, err
			}
		}
		for _, m := range models {
			if m.IsDefault {
				return m, nil
			}
		}
		if len(models) > 0 {
			return models[0], nil
		}
		return nil, nil
	}
	id, err := strconv.ParseUint(modelID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("无效的模型 ID: %s", modelID)
	}
	return modelRepo.FindByID(uint(id))
}
