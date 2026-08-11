package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// duckduckgoEngine DuckDuckGo 引擎：免费无需 key，走 HTML 端点解析结果
// 注意：海外服务，国内网络可能不可达，可在 provider 配置代理（ProxyURL）
type duckduckgoEngine struct {
	client *http.Client
}

func newDuckDuckGoEngine(client *http.Client) *duckduckgoEngine {
	return &duckduckgoEngine{client: client}
}

// Search 请求 html.duckduckgo.com/html 并解析结果块
func (e *duckduckgoEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	reqURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DocMind/1.0)")

	client := clientWithProxy(e.client, opts.ProxyURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 DuckDuckGo 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo 返回异常状态码: %d", resp.StatusCode)
	}

	results, err := parseDuckDuckGoHTML(resp.Body)
	if err != nil {
		return nil, err
	}
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}
	return results, nil
}

// Test 用最小查询验证可达性与解析链路
func (e *duckduckgoEngine) Test(ctx context.Context, opts SearchOptions) error {
	results, err := e.Search(ctx, "test", opts)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("DuckDuckGo 可用但未返回结果（可能被限流）")
	}
	return nil
}

// parseDuckDuckGoHTML 解析 HTML 端点：<div class="result"> 内含
// <a class="result__a" href="//duckduckgo.com/l/?uddg=<encoded>&...">标题</a>
// 与 <a class="result__snippet">摘要</a>，真实 URL 在 uddg 参数中
func parseDuckDuckGoHTML(r io.Reader) ([]Result, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("解析 DuckDuckGo 响应失败: %w", err)
	}

	var results []Result
	var cur *Result
	walk := func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		class := attr(n, "class")
		switch {
		case n.Data == "div" && strings.Contains(class, "result"):
			// 结果块边界：收尾上一个，开启新的
			if cur != nil {
				results = append(results, *cur)
			}
			cur = &Result{}
		case n.Data == "a" && strings.Contains(class, "result__a") && cur != nil:
			cur.Title = strings.TrimSpace(nodeText(n))
			if href := attr(n, "href"); href != "" {
				cur.URL = decodeDuckDuckGoURL(href)
			}
		case n.Data == "a" && strings.Contains(class, "result__snippet") && cur != nil:
			cur.Snippet = strings.TrimSpace(nodeText(n))
		}
	}
	walkHTML(doc, walk)
	if cur != nil {
		results = append(results, *cur)
	}

	// 过滤空条目（非结果容器 div 可能误判）
	filtered := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Title != "" && r.URL != "" {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// decodeDuckDuckGoURL 提取跳转链接中的真实 URL（uddg 参数），失败时原样返回
func decodeDuckDuckGoURL(href string) string {
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if uddg := u.Query().Get("uddg"); uddg != "" {
		if decoded, derr := url.QueryUnescape(uddg); derr == nil {
			return decoded
		}
		return uddg
	}
	return href
}

// walkHTML 深度优先遍历节点树
func walkHTML(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkHTML(c, fn)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return sb.String()
}
