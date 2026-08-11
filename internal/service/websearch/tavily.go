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

// tavilyEngine Tavily 引擎：官方 JSON API，需 api_key，结果质量稳定
// 文档：https://docs.tavily.com
type tavilyEngine struct {
	client *http.Client
}

func newTavilyEngine(client *http.Client) *tavilyEngine {
	return &tavilyEngine{client: client}
}

// tavilySearchRequest Tavily /search 请求体
type tavilySearchRequest struct {
	APIKey      string `json:"api_key"`
	Query       string `json:"query"`
	MaxResults  int    `json:"max_results"`
	SearchDepth string `json:"search_depth"`
}

// tavilySearchResponse Tavily /search 响应体
type tavilySearchResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// Search 调用 https://api.tavily.com/search
func (e *tavilyEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return nil, fmt.Errorf("Tavily 引擎需要配置 API Key")
	}
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	body, err := json.Marshal(tavilySearchRequest{
		APIKey:      opts.APIKey,
		Query:       query,
		MaxResults:  maxResults,
		SearchDepth: "basic",
	})
	if err != nil {
		return nil, err
	}

	endpoint := opts.BaseURL
	if endpoint == "" {
		endpoint = "https://api.tavily.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(endpoint, "/")+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := clientWithProxy(e.client, opts.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Tavily 失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily 返回异常状态码 %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var payload tavilySearchResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析 Tavily 响应失败: %w", err)
	}

	results := make([]Result, 0, len(payload.Results))
	for _, r := range payload.Results {
		results = append(results, Result{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}
	return results, nil
}

// Test 最小查询验证 API Key 有效性
func (e *tavilyEngine) Test(ctx context.Context, opts SearchOptions) error {
	results, err := e.Search(ctx, "test", opts)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("Tavily 可用但未返回结果")
	}
	return nil
}
