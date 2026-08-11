package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// baiduEngine 百度 AI 搜索引擎（千帆 AI 应用开发者中心「百度搜索」）
// 认证：单个 API Key（bce-v3/ALTAK-xxx 格式），Authorization: Bearer <API Key>
// 端点：POST https://qianfan.baidubce.com/v2/ai_search/web_search
// 文档：https://cloud.baidu.com/doc/qianfan-api/s/Wmbq4z7e5
// 计费：每月 1500 次免费额度（按天发放），支持按量后付费
type baiduEngine struct {
	client *http.Client
}

func newBaiduEngine(client *http.Client) *baiduEngine {
	return &baiduEngine{client: client}
}

// baiduSearchRequest 百度搜索请求体（仅网页模态，top_k 由调用方传入）
type baiduSearchRequest struct {
	Messages []baiduMessage `json:"messages"`
	// 固定值：百度搜索 v2
	SearchSource string `json:"search_source"`
	// 仅网页模态，top_k 最大 50
	ResourceTypeFilter []baiduResourceType `json:"resource_type_filter"`
}

type baiduMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type baiduResourceType struct {
	Type string `json:"type"`
	TopK int    `json:"top_k"`
}

// baiduSearchResponse 百度搜索响应
type baiduSearchResponse struct {
	// 成功时返回 references；失败时返回 code + message（HTTP 仍可能是 200）
	RequestID  string `json:"request_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	References []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet"`
		Content string `json:"content"`
	} `json:"references"`
}

// Search 调用千帆「百度搜索」接口（单轮查询）
func (e *baiduEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("百度引擎需要配置 API Key（千帆 bce-v3/ALTAK-xxx 格式）")
	}
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 50 {
		maxResults = 50 // 网页模态 top_k 上限
	}

	body, err := json.Marshal(baiduSearchRequest{
		Messages:           []baiduMessage{{Role: "user", Content: query}},
		SearchSource:       "baidu_search_v2",
		ResourceTypeFilter: []baiduResourceType{{Type: "web", TopK: maxResults}},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://qianfan.baidubce.com/v2/ai_search/web_search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(opts.APIKey))

	client := clientWithProxy(e.client, opts.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求百度搜索失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var payload baiduSearchResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析百度搜索响应失败: %w", err)
	}
	// 业务错误：HTTP 200 但 code 非空（如 216003 鉴权失败）
	if payload.Code != "" || payload.Message != "" {
		msg := payload.Message
		if msg == "" {
			msg = payload.Code
		}
		return nil, fmt.Errorf("百度搜索失败（%s）: %s", payload.Code, truncate(msg, 200))
	}

	results := make([]Result, 0, len(payload.References))
	for _, ref := range payload.References {
		snippet := ref.Snippet
		if snippet == "" {
			snippet = ref.Content
		}
		results = append(results, Result{
			Title:   ref.Title,
			URL:     ref.URL,
			Snippet: snippet,
		})
	}
	return results, nil
}

// Test 最小查询验证 API Key 有效性
func (e *baiduEngine) Test(ctx context.Context, opts SearchOptions) error {
	results, err := e.Search(ctx, "测试", opts)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("百度搜索可用但未返回结果")
	}
	return nil
}
