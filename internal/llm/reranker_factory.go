package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"
	"docmind/internal/repository"
)

// rerankerFactory 根据数据库中的模型配置创建 Reranker 实例
type rerankerFactory struct {
	modelRepo  repository.ModelRepository
	httpClient *http.Client
}

// NewRerankerFactory 创建 Rerank 工厂
func NewRerankerFactory(modelRepo repository.ModelRepository, httpClient *http.Client) pipeline.PipelineRerankerFactory {
	return &rerankerFactory{
		modelRepo:  modelRepo,
		httpClient: httpClient,
	}
}

// CreateReranker 根据 Rerank 模型 ID 创建 Reranker 实例
func (f *rerankerFactory) CreateReranker(ctx context.Context, modelID string) (pipeline.PipelineReranker, error) {
	model, err := f.resolveModel(modelID)
	if err != nil {
		return nil, err
	}

	return &httpReranker{
		model:      model,
		httpClient: f.httpClient,
	}, nil
}

// resolveModel 根据模型 ID 查找 Rerank 模型记录
func (f *rerankerFactory) resolveModel(modelID string) (*entity.Model, error) {
	// 尝试数字 ID 解析
	if id, err := strconv.ParseUint(modelID, 10, 64); err == nil {
		model, err := f.modelRepo.FindByID(uint(id))
		if err != nil {
			return nil, fmt.Errorf("查询模型失败: %w", err)
		}
		if model != nil {
			return model, nil
		}
	}
	// 按名称/显示名匹配
	models, err := f.modelRepo.List(entity.ModelTypeRerank, 0)
	if err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	for _, m := range models {
		if m.Name == modelID || m.DisplayName == modelID {
			return m, nil
		}
	}
	// 回退到默认模型
	for _, m := range models {
		if m.IsDefault {
			return m, nil
		}
	}
	return nil, fmt.Errorf("未找到 Rerank 模型: %s", modelID)
}

// httpReranker 通过 HTTP 调用 Rerank API 的实现
type httpReranker struct {
	model      *entity.Model
	httpClient *http.Client
}

// rerankRequest Rerank API 请求体（兼容 Jina / Cohere / 通用格式）
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResponse Rerank API 响应体
type rerankResponse struct {
	Results []rerankResultItem `json:"results"`
}

// rerankResultItem 单条 Rerank 结果
type rerankResultItem struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

// Rerank 执行 Rerank 重排序
func (r *httpReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]pipeline.PipelineRerankResult, error) {
	if r.model.Parameters.Provider == "docmindcloud" {
		// DocMindCloud 暂不支持真实 Rerank，直接返回原始顺序
		results := make([]pipeline.PipelineRerankResult, len(documents))
		for i := range documents {
			results[i] = pipeline.PipelineRerankResult{
				Index:          i,
				RelevanceScore: 1.0 - float64(i)*0.1,
			}
		}
		return results, nil
	}

	// 构建候选 URL：BaseURL 本身 + 常见后缀
	baseURL := strings.TrimRight(r.model.Parameters.BaseURL, "/")
	candidates := []string{baseURL}
	if !strings.Contains(strings.ToLower(baseURL), "rerank") {
		candidates = append(candidates, appendRerankPath(baseURL, "rerank"))
		candidates = append(candidates, appendRerankPath(baseURL, "v1/rerank"))
	}

	payload := rerankRequest{
		Model:     resolveRerankModelName(r.model),
		Query:     query,
		Documents: documents,
		TopN:      topK,
	}

	var lastErr error
	for _, candidateURL := range deduplicateRerankURLs(candidates) {
		result, err := r.doRerankRequest(ctx, candidateURL, payload)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("Rerank 调用失败: %w", lastErr)
	}
	return nil, fmt.Errorf("未找到可用的 Rerank 接口")
}

// doRerankRequest 发送单次 Rerank HTTP 请求
func (r *httpReranker) doRerankRequest(ctx context.Context, url string, payload rerankRequest) ([]pipeline.PipelineRerankResult, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 设置认证头
	if apiKey := strings.TrimSpace(r.model.Parameters.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	// 附加自定义请求头
	for key, value := range r.model.Parameters.CustomHeaders {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		// 跳过保留头
		if strings.EqualFold(trimmedKey, "Authorization") || strings.EqualFold(trimmedKey, "Content-Type") {
			continue
		}
		req.Header.Set(trimmedKey, trimmedValue)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Rerank API 返回 %d: %s", resp.StatusCode, truncateString(string(body), 200))
	}

	var rerankResp rerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (body: %s)", err, truncateString(string(body), 200))
	}

	results := make([]pipeline.PipelineRerankResult, 0, len(rerankResp.Results))
	for _, item := range rerankResp.Results {
		results = append(results, pipeline.PipelineRerankResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
		})
	}
	return results, nil
}

// resolveRerankModelName 确定实际使用的模型名称
func resolveRerankModelName(model *entity.Model) string {
	modelName := strings.TrimSpace(model.Parameters.ModelName)
	if modelName == "" {
		modelName = model.Name
	}
	return modelName
}

// appendRerankPath 在 baseURL 后拼接路径，自动处理斜杠
func appendRerankPath(baseURL, suffix string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(suffix, "/")
}

// deduplicateRerankURLs 去重 URL 列表
func deduplicateRerankURLs(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))
	for _, u := range urls {
		normalized := strings.TrimRight(strings.ToLower(u), "/")
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, u)
		}
	}
	return result
}

// truncateString 截断字符串用于错误信息
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
