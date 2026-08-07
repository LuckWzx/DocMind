package tools

import (
	"sort"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"

	"github.com/cloudwego/eino/components/tool"
)

// Registry 工具注册表：按 Agent 配置组装工具集
type Registry struct {
	deps *pipeline.PipelineDeps
}

// NewRegistry 创建工具注册表（依赖与 RAG 管道共用，见 service.BuildPipelineDeps）
func NewRegistry(deps *pipeline.PipelineDeps) *Registry {
	return &Registry{deps: deps}
}

// Build 构建该 Agent 的工具集
//   - AllowedTools 白名单过滤（空 = 全部可用）
//   - KBSelectionMode == "all" 或未指定知识库 → 工具内按用户全量检索兜底
//   - 返回引用收集器：与本次构建的工具绑定，Run 结束后读取 → SSE references
func (r *Registry) Build(agent *entity.Agent, userID uint) ([]tool.BaseTool, *ResultCollector, error) {
	cfg := agent.Config
	collector := &ResultCollector{}

	// 知识库范围：显式指定则固定，否则留空由 kb_search 工具全量兜底
	kbIDs := cfg.KnowledgeBases
	if cfg.KBSelectionMode == "all" {
		kbIDs = nil
	}
	searchCfg := &pipeline.SearchKBParams{
		KnowledgeBaseIDs: kbIDs,
		EmbeddingTopK:    cfg.EmbeddingTopK,
		VectorThreshold:  cfg.VectorThreshold,
		RerankModelID:    cfg.RerankModelID,
		RerankTopK:       cfg.RerankTopK,
	}
	if cfg.RerankThreshold != nil {
		searchCfg.RerankThreshold = *cfg.RerankThreshold
	}

	// 可用工具构建表（后续 Skills / MCP / 甲的工具集在此挂载）
	builders := map[string]func() (tool.BaseTool, error){
		"kb_search": func() (tool.BaseTool, error) {
			return NewKBSearchTool(r.deps, userID, searchCfg, collector)
		},
	}

	// AllowedTools 白名单过滤（空 = 全部可用）
	enabled := builders
	if len(cfg.AllowedTools) > 0 {
		enabled = make(map[string]func() (tool.BaseTool, error), len(cfg.AllowedTools))
		for _, name := range cfg.AllowedTools {
			if build, ok := builders[name]; ok {
				enabled[name] = build
			}
		}
	}

	// 按名称排序构建，保证工具注册顺序稳定
	names := make([]string, 0, len(enabled))
	for name := range enabled {
		names = append(names, name)
	}
	sort.Strings(names)

	built := make([]tool.BaseTool, 0, len(names))
	for _, name := range names {
		t, err := enabled[name]()
		if err != nil {
			return nil, nil, err
		}
		built = append(built, t)
	}
	return built, collector, nil
}
