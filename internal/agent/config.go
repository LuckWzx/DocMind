package agent

import (
	"context"
	"fmt"

	"docmind/internal/agent/skills"
	"docmind/internal/llm"
	"docmind/internal/model/entity"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// EngineConfig Agent 引擎配置（从 entity.AgentConfig 映射，规划 3.2.2）
type EngineConfig struct {
	Name            string
	Description     string
	Instruction     string // SystemPrompt
	ModelID         string
	Temperature     *float64
	MaxTokens       *int
	MaxIterations   int
	LLMCallTimeout  int
	AllowedTools    []string // 工具白名单（骨架阶段仅记录，过滤在工具注册层实现）
	SkillsBaseDir   string   // 技能目录（空 = 不启用技能系统）
	SelectedSkills  []string // 技能白名单（nil = 全部可用；空切片 = 全部禁用）
	EnableStreaming bool
}

// NewEngineConfig 从 Agent 实体映射引擎配置
// 骨架阶段覆盖基础字段；Skills / 检索工具 / MCP 等在后续迭代接入
func NewEngineConfig(a *entity.Agent) *EngineConfig {
	cfg := a.Config
	c := &EngineConfig{
		Name:            a.Name,
		Description:     a.Description,
		Instruction:     cfg.SystemPrompt,
		ModelID:         cfg.ModelID,
		Temperature:     cfg.Temperature,
		MaxTokens:       cfg.MaxCompletionTokens,
		AllowedTools:    cfg.AllowedTools,
		MaxIterations:   5, // 默认最大迭代步数
		EnableStreaming: true,
	}
	if cfg.MaxIterations != nil && *cfg.MaxIterations > 0 {
		c.MaxIterations = *cfg.MaxIterations
	}
	if cfg.LLMCallTimeout != nil {
		c.LLMCallTimeout = *cfg.LLMCallTimeout
	}
	// 技能系统：all（默认）→ 全部技能可用；manual → 按 SelectedSkills 白名单过滤
	c.SkillsBaseDir = skills.DefaultSkillsDir
	switch cfg.SkillsSelectionMode {
	case "manual":
		c.SelectedSkills = cfg.SelectedSkills
	default:
		c.SelectedSkills = nil
	}
	return c
}

// BuildAgentConfig 构建 adk.ChatModelAgentConfig
// ChatModel 由 llm.ChatModelFactory 创建（复用阶段一模型配置体系）
func (c *EngineConfig) BuildAgentConfig(ctx context.Context, factory *llm.ChatModelFactory, tools []tool.BaseTool) (*adk.ChatModelAgentConfig, error) {
	if c.ModelID == "" {
		return nil, fmt.Errorf("Agent 未配置模型(ModelID)")
	}
	chatModel, err := factory.CreateChatModel(ctx, c.ModelID)
	if err != nil {
		return nil, fmt.Errorf("创建 ChatModel 失败: %w", err)
	}
	// 技能系统：挂 skill middleware（默认工具名 skill，参数 {skill: 名称}）
	var handlers []adk.ChatModelAgentMiddleware
	if c.SkillsBaseDir != "" {
		skillHandler, err := skills.LoadSkillMiddleware(ctx, c.SkillsBaseDir, c.SelectedSkills)
		if err != nil {
			return nil, fmt.Errorf("加载技能系统失败: %w", err)
		}
		handlers = append(handlers, skillHandler)
	}
	return &adk.ChatModelAgentConfig{
		Name:          c.Name,
		Description:   c.Description,
		Instruction:   c.Instruction,
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: c.MaxIterations,
		Handlers:      handlers,
	}, nil
}
