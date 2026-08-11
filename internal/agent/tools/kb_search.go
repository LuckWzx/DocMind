package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// ResultCollector 检索引用收集器
// 每次工具构建绑定一个实例，Run 结束后读取 → SSE references 事件（规划 3.2.6 ⑥）
type ResultCollector struct {
	mu   sync.Mutex
	refs []entity.Reference
}

// Add 追加引用（工具执行线程内调用）
func (c *ResultCollector) Add(refs ...entity.Reference) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refs = append(c.refs, refs...)
}

// All 返回收集到的全部引用（Run 结束后读取）
func (c *ResultCollector) All() []entity.Reference {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]entity.Reference, len(c.refs))
	copy(out, c.refs)
	return out
}

// KBSearchArgs kb_search 工具参数（模型按 JSON Schema 填充）
type KBSearchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"` // 可选，0 = 使用 Agent 配置的条数
}

// kbSearchHit 工具返回的单条命中
type kbSearchHit struct {
	ChunkID        uint    `json:"chunk_id"`
	KnowledgeTitle string  `json:"knowledge_title"`
	Content        string  `json:"content"`
	Score          float64 `json:"score"`
}

// kbSearchOut 工具返回给模型的结构化结果
type kbSearchOut struct {
	Total int           `json:"total"`
	Hits  []kbSearchHit `json:"hits"`
}

// NewKBSearchTool 构建知识库检索工具
// 闭包捕获：检索依赖、用户上下文（UserID + 知识库范围 + 检索参数）、引用收集器
// 知识库范围为空时检索该用户全部知识库（与 resolveAgentConfig 兜底语义一致）
func NewKBSearchTool(
	deps *pipeline.PipelineDeps,
	userID uint,
	searchCfg *pipeline.SearchKBParams,
	collector *ResultCollector,
) (tool.BaseTool, error) {
	searchFn := func(ctx context.Context, args KBSearchArgs) (string, error) {
		// 1. 解析知识库范围：未指定 → 检索用户全部知识库
		kbIDs := searchCfg.KnowledgeBaseIDs
		if len(kbIDs) == 0 {
			if all, err := deps.KBRepo.ListByUser(userID); err == nil {
				for _, kb := range all {
					kbIDs = append(kbIDs, fmt.Sprintf("%d", kb.ID))
				}
			}
		}
		if len(kbIDs) == 0 {
			return "未找到相关资料：当前用户没有可检索的知识库。", nil
		}

		// 2. 执行检索（复用 pipeline 检索段：embedding → pgvector → rerank）
		topK := args.TopK
		if topK <= 0 {
			topK = searchCfg.EmbeddingTopK
		}
		results, err := pipeline.SearchKB(ctx, deps, &pipeline.SearchKBParams{
			UserID:           userID,
			KnowledgeBaseIDs: kbIDs,
			Query:            args.Query,
			EmbeddingTopK:    topK,
			VectorThreshold:  searchCfg.VectorThreshold,
			KeywordTopK:      searchCfg.KeywordTopK,
			KeywordThreshold: searchCfg.KeywordThreshold,
			RerankModelID:    searchCfg.RerankModelID,
			RerankTopK:       searchCfg.RerankTopK,
			RerankThreshold:  searchCfg.RerankThreshold,
		})
		if err != nil {
			// 检索失败不中断 Agent：返回降级文案，由模型组织降级回答（规划 3.2.6 ⑤）
			// 注意：文案避免"请稍后重试"等重试暗示，防止模型反复调用工具耗尽迭代上限
			return fmt.Sprintf("知识库检索失败（错误：%v），本次无法获取知识库资料，请基于已有知识直接回答，不要编造。", err), nil
		}
		if len(results) == 0 {
			return "未找到相关资料：知识库中没有与问题相关的内容，请基于已有知识回答，不要编造。", nil
		}

		// 3. 收集引用（SSE references 事件数据源）
		refs := make([]entity.Reference, 0, len(results))
		hits := make([]kbSearchHit, 0, len(results))
		for _, r := range results {
			refs = append(refs, entity.Reference{
				ChunkID:        r.ChunkID,
				Content:        r.Content,
				Score:          r.Score,
				KnowledgeID:    r.KnowledgeID,
				KnowledgeTitle: r.KnowledgeTitle,
			})
			hits = append(hits, kbSearchHit{
				ChunkID:        r.ChunkID,
				KnowledgeTitle: r.KnowledgeTitle,
				Content:        r.Content,
				Score:          r.Score,
			})
		}
		collector.Add(refs...)

		// 4. 结构化返回（模型据此组织回答，引用溯源）
		data, err := json.Marshal(kbSearchOut{Total: len(hits), Hits: hits})
		if err != nil {
			return "", fmt.Errorf("序列化检索结果失败: %w", err)
		}
		return string(data), nil
	}

	return utils.InferTool[KBSearchArgs, string](
		"kb_search",
		"在企业知识库中检索相关资料。当用户问题涉及知识库内容（制度、文档、数据等）时必须调用；"+
			"参数 query 为检索问题，top_k 为可选返回条数（默认按配置）。",
		searchFn,
	)
}
