package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"docmind/internal/model/entity"
	"docmind/internal/service/websearch"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// WebSearchService web_search 工具所需的最小服务接口（消费方定义，避免 tools→service 循环依赖；
// 运行时由 service.WebSearchService 注入，方法集超集可安全赋值）
type WebSearchService interface {
	// ResolveForAgent 解析用户可用提供方：显式 ID 优先，兜底默认/首个启用（nil = 无可用）
	ResolveForAgent(userID, providerID uint) (*entity.WebSearchProvider, error)
	// Search 按提供方配置执行搜索
	Search(ctx context.Context, provider *entity.WebSearchProvider, query string, maxResults int) ([]websearch.Result, error)
}

// WebSearchArgs web_search 工具参数（模型按 JSON Schema 填充）
type WebSearchArgs struct {
	Query string `json:"query"`
	// MaxResults 可选返回条数，0 = 使用 Agent 配置的条数
	MaxResults int `json:"max_results,omitempty"`
}

// webSearchHit 工具返回的单条命中
type webSearchHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// webSearchOut 工具返回给模型的结构化结果（search_source 供前端检索来源展示）
type webSearchOut struct {
	SearchSource string         `json:"search_source"`
	Count        int            `json:"count"`
	Results      []webSearchHit `json:"results"`
}

// NewWebSearchTool 构建网页搜索工具
// 闭包捕获：WebSearchService、用户上下文（UserID + provider 选择 + 条数配置）
// provider 未配置时不报错：返回降级文案，由模型基于已有知识回答
func NewWebSearchTool(
	svc WebSearchService,
	userID uint,
	providerID uint,
	maxResults int,
) (tool.BaseTool, error) {
	searchFn := func(ctx context.Context, args WebSearchArgs) (string, error) {
		// 1. 解析提供方：显式 ID 优先，兜底默认/首个启用（与 ResolveForAgent 语义一致）
		provider, err := svc.ResolveForAgent(userID, providerID)
		if err != nil {
			return fmt.Sprintf("网页搜索失败（错误：%v），本次无法获取网络资料，请基于已有知识直接回答，不要编造。", err), nil
		}
		if provider == nil {
			return "未配置网页搜索提供方（请先在设置中配置搜索引擎），本次无法获取网络资料，请基于已有知识直接回答，不要编造。", nil
		}

		// 2. 执行搜索
		topK := args.MaxResults
		if topK <= 0 {
			topK = maxResults
		}
		results, err := svc.Search(ctx, provider, args.Query, topK)
		if err != nil {
			// 搜索失败不中断 Agent：返回降级文案，由模型组织降级回答
			// 注意：文案避免"请稍后重试"等重试暗示，防止模型反复调用工具耗尽迭代上限
			return fmt.Sprintf("网页搜索失败（错误：%v），本次无法获取网络资料，请基于已有知识直接回答，不要编造。", err), nil
		}
		if len(results) == 0 {
			return "未找到相关网页内容，请基于已有知识回答，不要编造。", nil
		}

		// 3. 结构化返回（模型据此组织回答，附来源 URL）
		hits := make([]webSearchHit, 0, len(results))
		for _, r := range results {
			hits = append(hits, webSearchHit{Title: r.Title, URL: r.URL, Snippet: r.Snippet})
		}
		data, err := json.Marshal(webSearchOut{SearchSource: "web", Count: len(hits), Results: hits})
		if err != nil {
			return "", fmt.Errorf("序列化搜索结果失败: %w", err)
		}
		return string(data), nil
	}

	return utils.InferTool[WebSearchArgs, string](
		"web_search",
		"在互联网上搜索最新信息（实时新闻、最新动态、知识库无法覆盖的内容）。\n"+
			"## 调用纪律（必须遵守）\n"+
			"1. 如果配置了知识库检索工具（如 kb_search），必须先完成知识库检索，再决定是否使用本工具；\n"+
			"2. 仅在知识库检索无结果或结果不足、且用户问题需要最新/实时信息时才调用本工具；\n"+
			"3. 不要用本工具检索知识库中已有明确答案的内容，避免重复检索；\n"+
			"4. 若搜索失败或结果为空，直接基于已有知识回答，并说明无法获取网络信息，不要反复调用本工具。\n"+
			"参数 query 为搜索关键词，max_results 为可选返回条数（默认按配置）。",
		searchFn,
	)
}

// parseUintID 解析字符串 ID（空/非法返回 0）
func parseUintID(idStr string) uint {
	if strings.TrimSpace(idStr) == "" {
		return 0
	}
	id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
	if err != nil {
		return 0
	}
	return uint(id)
}

// toolMaxResults 读取 Agent 配置的最大搜索结果数（未配置默认 5）
func toolMaxResults(cfg entity.AgentConfig) int {
	if cfg.WebSearchMaxResults != nil && *cfg.WebSearchMaxResults > 0 {
		return *cfg.WebSearchMaxResults
	}
	return 5
}
