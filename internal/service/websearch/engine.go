package websearch

import (
	"context"
	"net/http"
	"net/url"
)

// 引擎类型常量（与前端 WebSearchProviderEntity.provider 对齐）
const (
	EngineDuckDuckGo = "duckduckgo"
	EngineTavily     = "tavily"
	EngineBaidu      = "baidu"
)

// Result 统一搜索结果条目
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchOptions 单次搜索参数（由 provider 配置填充）
type SearchOptions struct {
	MaxResults int
	APIKey     string // tavily api_key / baidu 千帆 API Key
	BaseURL    string // 自建服务地址（可选）
	ProxyURL   string // 代理地址（可选）
	Extra      map[string]string
}

// Engine 搜索引擎适配接口：不同引擎实现各自协议，统一输出 Result
type Engine interface {
	// Search 执行搜索，返回按相关性排序的结果（最多 MaxResults 条）
	Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error)
	// Test 连通性测试：用最小请求验证凭据/网络可用（provider 配置测试用）
	Test(ctx context.Context, opts SearchOptions) error
}

// parseProxy 解析代理地址（引擎请求时注入 http.Transport.Proxy）
func parseProxy(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// clientWithProxy 克隆基础 Transport 并注入代理，返回独立 Client
// 避免并发修改共享 Transport.Proxy 的竞态；搜索频率低，克隆开销可接受
func clientWithProxy(base *http.Client, proxyURL string) *http.Client {
	if proxyURL == "" {
		return base
	}
	transport, ok := base.Transport.(*http.Transport)
	if !ok || transport == nil {
		return base
	}
	clone := transport.Clone()
	if u, err := parseProxy(proxyURL); err == nil {
		clone.Proxy = http.ProxyURL(u)
	}
	return &http.Client{Transport: clone, Timeout: base.Timeout}
}

// truncate 截断字符串（错误信息防超长）
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
