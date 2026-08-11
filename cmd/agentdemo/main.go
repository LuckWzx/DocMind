// 最小 Demo：验证 internal/agent 引擎骨架 + 真实知识库检索工具 + 技能系统（模块5）
//
// 验证点：
//  1. entity.Agent → NewEngineConfig → BuildAgentConfig（adk.ChatModelAgentConfig）映射链路
//  2. BuildAgentConfig 内部复用 ChatModelFactory（internal/llm）创建 ToolCallingChatModel
//  3. tools.Registry 按 Agent 配置构建工具集（AllowedTools 白名单）
//  4. kb_search 工具走真实检索链路：pipeline.SearchKB（向量 ‖ BM25 → RRF → rerank）
//  5. 技能系统：skills.LoadSkillMiddleware 挂 Handlers，模型可调用 skill 工具加载 SKILL.md 指令
//  6. ResultCollector 收集引用（SSE references 事件数据源）
//
// 运行：go run ./cmd/agentdemo
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"docmind/internal/agent"
	"docmind/internal/agent/tools"
	"docmind/internal/llm"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	"docmind/internal/service"
	"docmind/pkg/config"
	"docmind/pkg/database"
	"docmind/pkg/logger"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// ===== 工具 1：calculator（普通工具，与 kb_search 并列验证工具分流）=====

type CalculatorArgs struct {
	A  int    `json:"a"`
	B  int    `json:"b"`
	Op string `json:"op"`
}

func calcFn(_ context.Context, args CalculatorArgs) (string, error) {
	switch args.Op {
	case "+":
		return fmt.Sprintf("%d", args.A+args.B), nil
	case "-":
		return fmt.Sprintf("%d", args.A-args.B), nil
	case "*":
		return fmt.Sprintf("%d", args.A*args.B), nil
	case "/":
		if args.B == 0 {
			return "", fmt.Errorf("除数不能为 0")
		}
		return fmt.Sprintf("%d", args.A/args.B), nil
	default:
		return "", fmt.Errorf("不支持的运算符: %s", args.Op)
	}
}

// ===== 工具 2：mock_kb_search 已移除，改为 tools.Registry 构建的真实 kb_search =====

func main() {
	ctx := context.Background()

	// 1. 配置 + 数据库（模型配置存在数据库中）
	cfg, err := config.Load("")
	must(err)
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Printf("❌ 日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	db, err := database.InitPostgreSQL(&cfg.Database.PostgreSQL)
	must(err)

	// 2. 模型工厂：从数据库选一个可用的对话模型
	modelRepo := repository.NewModelRepository(db)
	factory := llm.NewChatModelFactory(modelRepo)

	models, err := modelRepo.ListAll(entity.ModelTypeKnowledgeQA)
	must(err)
	if len(models) == 0 {
		fmt.Println("❌ 数据库中没有 KnowledgeQA 类型模型，请先在模型管理中配置")
		os.Exit(1)
	}
	for _, m := range models {
		fmt.Printf("  候选模型: id=%d name=%s model_name=%q status=%s provider=%s\n",
			m.ID, m.Name, m.Parameters.ModelName, m.Status, m.Parameters.Provider)
	}
	picked := models[0]
	for _, m := range models {
		if m.Status == entity.ModelStatusActive && m.Parameters.BaseURL != "" &&
			m.Parameters.APIKey != "" && m.Parameters.ModelName != "" {
			picked = m
			break
		}
	}
	fmt.Printf("✅ 使用模型: %s (provider=%s, model=%s, url=%s)\n",
		picked.Name, picked.Parameters.Provider, picked.Parameters.ModelName, picked.Parameters.BaseURL)

	// 3. 检索依赖（与 chat_service 共用一套：service.BuildPipelineDeps）
	embedderFactory := llm.NewEmbedderFactory(modelRepo)
	rerankerFactory := llm.NewRerankerFactory(modelRepo, &http.Client{Timeout: 30 * time.Second})
	kbRepo := repository.NewKnowledgeBaseRepository(db)
	vectorStoreRepo := repository.NewVectorStoreRepository(db)
	pipelineDeps := service.BuildPipelineDeps(embedderFactory, rerankerFactory, kbRepo, vectorStoreRepo, db, false)

	// 检索用户上下文：优先选有规范检索数据（kb_id>0 且全局默认 store）的用户
	var defaultStoreID uint
	if err := db.Table("vector_stores").
		Select("id").
		Where("user_id = 0 AND engine_type = ?", "postgres").
		Order("id ASC").Limit(1).Scan(&defaultStoreID).Error; err != nil {
		must(err)
	}
	var userID uint
	err = db.Table("chunk_vectors").
		Select("user_id").
		Where("is_enabled = ? AND knowledge_base_id > 0 AND vector_store_id = ?", true, defaultStoreID).
		Group("user_id").Order("user_id ASC").Limit(1).Scan(&userID).Error
	if err != nil || userID == 0 {
		// 兜底：取系统第一个用户
		if err := db.Table("users").Select("id").Order("id ASC").Limit(1).Scan(&userID).Error; err != nil || userID == 0 {
			fmt.Println("❌ 数据库中没有用户，请先注册用户")
			os.Exit(1)
		}
	}
	fmt.Printf("✅ 检索用户: id=%d（chunk_vectors.user_id 过滤）\n", userID)

	// 4. Agent 实体（模拟内置智能推理 Agent 的配置，验证 实体 → 引擎配置 映射）
	agt := &entity.Agent{
		Name:        "demo-agent",
		Description: "DocMind 最小 Demo Agent",
		Config: entity.AgentConfig{
			AgentMode: "smart-reasoning",
			SystemPrompt: "你是一个企业知识库问答助手。涉及数学计算时调用 calculator 工具；" +
				"涉及公司制度/报销/请假等知识库问题时调用 kb_search 工具；" +
				"用户要求给出 RAG 优化/文档评审等专业方法论时，先调用 skill 工具加载对应技能再回答；" +
				"根据检索结果组织最终回答，引用来源文档，不要编造工具未提供的信息。",
			ModelID:       strconv.FormatUint(uint64(picked.ID), 10),
			MaxIterations: intPtr(6),
			EmbeddingTopK: 5,
			// 多轮对话：引擎自动加载会话历史（第二轮验证 HistoryProvider）
			MultiTurnEnabled: boolPtr(true),
			HistoryTurns:     3,
			// KnowledgeBases 为空 + KBSelectionMode 为空 → kb_search 全量检索用户知识库
		},
	}

	// 5. 工具集：Registry 按 Agent 配置构建（AllowedTools 白名单；Demo 不挂载 MCP，传 nil）
	calcTool, err := utils.InferTool[CalculatorArgs, string]("calculator", "计算两个整数的四则运算，参数 a、b 必须为整数，op 为运算符(+ - * /)", calcFn)
	must(err)
	registry := tools.NewRegistry(pipelineDeps, nil, nil, nil) // Demo 不挂载 MCP/web_search，传 nil
	builtTools, collector, err := registry.Build(agt, userID)
	must(err)
	allTools := append(builtTools, calcTool)
	for _, t := range allTools {
		info, _ := t.Info(ctx)
		fmt.Printf("🔧 注册工具: %s\n", info.Name)
	}

	// 6. 引擎：配置映射 → ADK Agent 构建 → 引擎实例
	engineCfg := agent.NewEngineConfig(agt)
	adkCfg, err := engineCfg.BuildAgentConfig(ctx, factory, allTools)
	must(err)
	chatAgent, err := adk.NewChatModelAgent(ctx, adkCfg)
	must(err)
	eng := agent.NewEngine(chatAgent, true)

	// 7. 第一轮提问：触发技能（skill 工具） + 真实知识库检索（kb_search）
	question1 := "帮我用 RAG 优化顾问技能诊断一下：检索效果不佳时应该怎么优化？顺便从知识库检索一下 RAG 优化方案有哪些"
	answer1, steps1 := runQuestion(eng, &agent.RunRequest{
		SessionID: 1,
		UserID:    userID,
		Messages:  []*schema.Message{{Role: schema.User, Content: question1}},
		Agent:     agt,
	}, question1)
	printSteps("第一轮", steps1)

	// 8. 第二轮提问：验证多轮历史自动加载（Messages 为空 + History + Question）
	// 引擎按 Agent.Config.MultiTurnEnabled/HistoryTurns 自动加载历史并追加当前问题
	historyMsgs := []*schema.Message{{Role: schema.User, Content: question1}}
	if answer1 != "" {
		historyMsgs = append(historyMsgs, &schema.Message{Role: schema.Assistant, Content: answer1})
	}
	history := &memHistory{msgs: historyMsgs}
	question2 := "我刚才问你什么问题？你用了哪个技能？"
	_, steps2 := runQuestion(eng, &agent.RunRequest{
		SessionID: 1,
		UserID:    userID,
		Question:  question2,
		History:   history,
		Agent:     agt,
	}, question2)
	printSteps("第二轮", steps2)

	// 9. 引用溯源（ResultCollector → SSE references 数据源）
	refs := collector.All()
	fmt.Printf("\n📎 检索引用（ResultCollector）: %d 条\n", len(refs))
	for i, r := range refs {
		content := r.Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		fmt.Printf("  ref%d: [%s] score=%.3f %s\n", i+1, r.KnowledgeTitle, r.Score, content)
	}
	fmt.Println("\n✅ Demo 结束：internal/agent 引擎 + 真实 kb_search + 技能 + 状态机/步骤记录验证通过")
}

// ===== 事件消费与步骤展示 =====

// memHistory 内存版会话历史（模拟 chat_service 注入的 repository 实现）
type memHistory struct {
	msgs []*schema.Message
}

// LoadHistory 实现 agent.HistoryProvider
func (h *memHistory) LoadHistory(_ context.Context, _ uint, _ int) ([]*schema.Message, error) {
	return h.msgs, nil
}

// runQuestion 执行一次提问并消费事件流，返回回答文本与引擎自动生成的步骤记录
func runQuestion(eng *agent.Engine, req *agent.RunRequest, question string) (string, entity.AgentSteps) {
	fmt.Printf("\n👤 提问: %s\n\n", question)
	stream, err := eng.Run(context.Background(), req)
	must(err)

	var sb strings.Builder
	start := time.Now()
	for {
		ev, ok := stream.Next()
		if !ok {
			fmt.Printf("\n[complete] 事件流结束，耗时 %dms\n", time.Since(start).Milliseconds())
			break
		}
		switch ev.Type {
		case agent.EventAnswer:
			fmt.Print(ev.Content)
			sb.WriteString(ev.Content)
		case agent.EventStep:
			if ev.ToolArgs != "" {
				fmt.Printf("[tool_call] %s(%s)\n", ev.ToolName, ev.ToolArgs)
			}
			if ev.ToolResult != "" {
				result := ev.ToolResult
				if len(result) > 120 {
					result = result[:120] + "..."
				}
				fmt.Printf("\n[agent_step] tool=%s result=%s\n", ev.ToolName, result)
			}
		case agent.EventState:
			fmt.Printf("[state] %s\n", ev.State)
		case agent.EventError:
			fmt.Printf("\n[error] %s\n", ev.Content)
		}
	}
	return sb.String(), stream.Steps()
}

// printSteps 打印引擎自动生成的步骤记录（entity.AgentSteps，规划 3.2.5：
// 事件流自动生成，含 Thought / ToolCalls（参数、结果、耗时））
func printSteps(label string, steps entity.AgentSteps) {
	fmt.Printf("\n📋 %s步骤记录（engine 自动生成）: %d 条\n", label, len(steps))
	for i, s := range steps {
		thought := s.Thought
		if len(thought) > 80 {
			thought = thought[:80] + "..."
		}
		if thought != "" {
			fmt.Printf("  step%d [%.0fms] 💭 %s\n", i+1, float64(s.Duration), thought)
		} else {
			fmt.Printf("  step%d [%.0fms]\n", i+1, float64(s.Duration))
		}
		for _, tc := range s.ToolCalls {
			result := ""
			if tc.Result != nil {
				result = tc.Result.Output
				if len(result) > 60 {
					result = result[:60] + "..."
				}
			}
			fmt.Printf("     🛠 %s(%s) [%.0fms] → %s\n", tc.Name, tc.Args, float64(tc.Duration), result)
		}
	}
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func must(err error) {
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
}
