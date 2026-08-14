package tools

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"docmind/internal/mcp"
	"docmind/internal/model/entity"
	"docmind/internal/pipeline"
	"docmind/internal/repository"
	"docmind/internal/sandbox"

	"github.com/cloudwego/eino/components/tool"
)

// Registry 工具注册表：按 Agent 配置组装工具集
// 内置工具（kb_search / web_search / data_analysis 等）+ 外部 MCP 工具（按 MCPServices 配置挂载）
type Registry struct {
	deps          *pipeline.PipelineDeps
	mcpRepo       repository.MCPServiceRepository
	approvalRepo  repository.MCPToolApprovalRepository
	mcpManager    *mcp.Manager
	webSearchSvc  WebSearchService
	knowledgeRepo repository.KnowledgeRepository // data_analysis/data_schema 数据源（可为 nil：不注册）
	sandbox       sandbox.Sandbox                // python_exec 沙箱（可为 nil：不注册）
}

// NewRegistry 创建工具注册表（依赖与 RAG 管道共用，见 service.BuildPipelineDeps）
// mcpRepo / mcpManager 可为 nil：不挂载 MCP 工具
// webSearchSvc 可为 nil：不注册 web_search 工具
// knowledgeRepo 可为 nil：不注册 data_analysis / data_schema 工具
// sb 可为 nil：不注册 python_exec 工具
func NewRegistry(deps *pipeline.PipelineDeps, mcpRepo repository.MCPServiceRepository, approvalRepo repository.MCPToolApprovalRepository, mcpManager *mcp.Manager, webSearchSvc WebSearchService, knowledgeRepo repository.KnowledgeRepository, sb sandbox.Sandbox) *Registry {
	return &Registry{
		deps:          deps,
		mcpRepo:       mcpRepo,
		approvalRepo:  approvalRepo,
		mcpManager:    mcpManager,
		webSearchSvc:  webSearchSvc,
		knowledgeRepo: knowledgeRepo,
		sandbox:       sb,
	}
}

// toolEnabled 判断工具是否在白名单内（空白名单 = 全部可用）
func toolEnabled(allowed []string, name string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == name {
			return true
		}
	}
	return false
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
		KeywordTopK:      cfg.KeywordTopK,
		RerankModelID:    cfg.RerankModelID,
		RerankTopK:       cfg.RerankTopK,
	}
	if cfg.RerankThreshold != nil {
		searchCfg.RerankThreshold = *cfg.RerankThreshold
	}
	if cfg.KeywordThreshold != nil {
		searchCfg.KeywordThreshold = *cfg.KeywordThreshold
	}

	// 可用工具构建表（后续 Skills / 甲的工具集在此挂载）
	builders := map[string]func() (tool.BaseTool, error){
		"kb_search": func() (tool.BaseTool, error) {
			return NewKBSearchTool(r.deps, userID, searchCfg, collector)
		},
	}
	// web_search 工具（开关驱动见前端 allTools 勾选；无 WebSearchService 时不注册）
	if r.webSearchSvc != nil {
		builders["web_search"] = func() (tool.BaseTool, error) {
			return NewWebSearchTool(r.webSearchSvc, userID, parseUintID(cfg.WebSearchProviderID), toolMaxResults(cfg))
		}
	}
	// 数据分析工具（data_analysis / data_schema）：共享请求级 DuckDB 内存库，
	// 生命周期挂到 collector.Cleanup()（controller 流结束后调用，见 ResultCollector.Cleanup）
	if r.knowledgeRepo != nil && (toolEnabled(cfg.AllowedTools, "data_analysis") || toolEnabled(cfg.AllowedTools, "data_schema")) {
		analysisSession, err := newAnalysisSession()
		if err != nil {
			return nil, nil, err
		}
		collector.SetCleanup(analysisSession.Close)
		builders["data_analysis"] = func() (tool.BaseTool, error) {
			return NewDataAnalysisTool(r.knowledgeRepo, r.deps.KBRepo, userID, searchCfg.KnowledgeBaseIDs, analysisSession)
		}
		builders["data_schema"] = func() (tool.BaseTool, error) {
			return NewDataSchemaTool(r.knowledgeRepo, r.deps.KBRepo, userID, searchCfg.KnowledgeBaseIDs, analysisSession)
		}
	}

	// Python 沙箱工具（python_exec）：模型生成代码 → 沙箱隔离执行（注册表持有沙箱时挂载）
	if r.sandbox != nil && toolEnabled(cfg.AllowedTools, "python_exec") {
		builders["python_exec"] = func() (tool.BaseTool, error) {
			return NewPythonExecTool(r.sandbox, r.knowledgeRepo, r.deps.KBRepo, userID, searchCfg.KnowledgeBaseIDs)
		}
	}

	// 构建全部工具（内置 + MCP），再统一按 AllowedTools 白名单过滤（空 = 全部可用）
	built := make([]tool.BaseTool, 0, len(builders))
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t, err := builders[name]()
		if err != nil {
			return nil, nil, err
		}
		built = append(built, t)
	}

	// MCP 工具挂载（MCPSelectionMode: manual 白名单 / 默认全部启用服务）
	mcpTools, err := r.buildMCPTools(agent, userID)
	if err != nil {
		return nil, nil, err
	}
	built = append(built, mcpTools...)

	// AllowedTools 白名单统一过滤（内置与 MCP 工具一并按名过滤）
	if len(cfg.AllowedTools) > 0 {
		allow := make(map[string]struct{}, len(cfg.AllowedTools))
		for _, name := range cfg.AllowedTools {
			allow[name] = struct{}{}
		}
		filtered := built[:0]
		for _, t := range built {
			info, err := t.Info(context.Background())
			if err != nil {
				continue
			}
			if _, ok := allow[info.Name]; ok {
				filtered = append(filtered, t)
			}
		}
		built = filtered
	}

	// 按名称排序，保证工具注册顺序稳定
	sort.SliceStable(built, func(i, j int) bool {
		infoI, errI := built[i].Info(context.Background())
		infoJ, errJ := built[j].Info(context.Background())
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.Name < infoJ.Name
	})
	return built, collector, nil
}

// buildMCPTools 按 Agent 配置挂载外部 MCP 工具：
//   - MCPSelectionMode "none"        → 不挂载任何 MCP 工具
//   - MCPSelectionMode "selected"/"manual" 且 MCPServices 非空 → 按服务 ID 白名单选择
//   - 其他（"all" / 未配置）→ 用户全部已启用服务
//
// 工具名统一加前缀 mcp_<service>_<tool>（见 prefixedTool），单个服务连接失败不影响其他服务
func (r *Registry) buildMCPTools(agent *entity.Agent, userID uint) ([]tool.BaseTool, error) {
	if r.mcpRepo == nil || r.mcpManager == nil {
		return nil, nil
	}
	cfg := agent.Config

	// 前端契约：all / selected / none（兼容旧值 manual）
	if cfg.MCPSelectionMode == "none" {
		return nil, nil
	}

	var svcs []*entity.MCPService
	if (cfg.MCPSelectionMode == "selected" || cfg.MCPSelectionMode == "manual") && len(cfg.MCPServices) > 0 {
		for _, idStr := range cfg.MCPServices {
			id, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 64)
			if err != nil {
				continue
			}
			svc, err := r.mcpRepo.FindByUserAndID(userID, uint(id))
			if err != nil || svc == nil || !svc.Enabled {
				continue
			}
			svcs = append(svcs, svc)
		}
	} else {
		var err error
		svcs, err = r.mcpRepo.ListEnabledByUser(userID)
		if err != nil {
			return nil, err
		}
	}
	if len(svcs) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	var out []tool.BaseTool
	// 工具名前缀去重：系统级与用户级可能存在同名服务（如用户自建 mcp_api_requester），
	// 工具名 mcp_<name>_<tool> 冲突时保留先注册者（系统级优先，ListEnabledByUser 按 user_id ASC 排序）
	seen := make(map[string]struct{}, len(svcs))
	// 审批偏好：一次拉取当前用户全部设置，构建 serviceID→toolName→requireApproval 映射
	approvals := map[uint]map[string]bool{}
	if r.approvalRepo != nil {
		if rows, err := r.approvalRepo.ListByUser(userID); err == nil {
			for _, row := range rows {
				if row.RequireApproval {
					if approvals[row.ServiceID] == nil {
						approvals[row.ServiceID] = map[string]bool{}
					}
					approvals[row.ServiceID][row.ToolName] = true
				}
			}
		}
	}
	for _, svc := range svcs {
		prefix := "mcp_" + sanitizeToolName(svc.Name)
		if _, ok := seen[prefix]; ok {
			// 同名前缀冲突（如多个中文名服务都兜底为 mcp_svc）：追加服务 ID 区分，
			// 避免后注册服务被误判为重复而静默丢失
			prefix = fmt.Sprintf("%s%d", prefix, svc.ID)
			if _, ok2 := seen[prefix]; ok2 {
				continue
			}
		}
		cli, err := r.mcpManager.GetClient(ctx, svc)
		if err != nil {
			fmt.Printf("[MCP] 服务[%d]%s 连接失败: %v\n", svc.ID, svc.Name, err)
			// 单个服务连接失败不影响其他服务挂载
			continue
		}
		einoTools, err := mcp.GetEinoTools(ctx, cli, mcp.BuildHeaders(svc))
		if err != nil {
			fmt.Printf("[MCP] 服务[%d]%s 工具列表拉取失败: %v\n", svc.ID, svc.Name, err)
			continue
		}
		seen[prefix] = struct{}{}
		svcApprovals := approvals[svc.ID]
		for _, t := range einoTools {
			info, err := t.Info(ctx)
			if err != nil {
				continue
			}
			out = append(out, &prefixedTool{inner: t, prefix: prefix, requireApproval: svcApprovals[info.Name]})
		}
	}
	return out, nil
}
