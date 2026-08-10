package longterm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// 长期记忆默认值
const (
	// DefaultRetrieveLimit 检索返回的最大 Episode 数（未配置时默认 5）
	DefaultRetrieveLimit = 5
	// DefaultMaxEpisodesPerSession 单个会话可存储的 Episode 上限（超出后新问答不再录入，防图库无限膨胀）
	DefaultMaxEpisodesPerSession = 200
	// DefaultExtractTimeout 单次 LLM 提取超时
	DefaultExtractTimeout = 60 * time.Second
	// DefaultExtractRetries LLM 提取失败重试次数（重试后仍失败则跳过本次记忆）
	DefaultExtractRetries = 1
)

// Episode 记忆片段：一次问答交互的摘要与来源信息。
// 对应 Neo4j 中的 Episode 节点。
type Episode struct {
	ID        string // 全局唯一 ID（uuid）
	UserID    uint
	SessionID uint
	Summary   string // LLM 生成的对话摘要（entity 提取失败时仍有此字段可检索）
	CreatedAt time.Time
}

// Entity 对话中提取出的实体（Neo4j Entity 节点，按 (name, user_id) 唯一）
type Entity struct {
	Title       string
	Type        string // 实体类型（Person / Concept / Project ...）
	Description string
}

// Relationship 实体间关系（Neo4j RELATED_TO 边）
type Relationship struct {
	Source      string // 源实体名
	Target      string // 目标实体名
	Description string
	Weight      float64 // 关系强度（0-1）
}

// MemoryContext 检索到的长期记忆上下文（注入对话 prompt 用）
type MemoryContext struct {
	RelatedEpisodes []*Episode
}

// Text 将检索结果格式化为可注入 user 消息尾部的文本。
// 无相关记忆时返回空串（调用方直接跳过拼接）。
func (c *MemoryContext) Text() string {
	if c == nil || len(c.RelatedEpisodes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nRelevant Memory:\n")
	for _, ep := range c.RelatedEpisodes {
		if ep == nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", ep.CreatedAt.Format("2006-01-02"), ep.Summary))
	}
	return sb.String()
}

// MemoryService 长期记忆服务接口（quick-answer 模式接入；Agent 引擎就绪后按同一接口挂载）
type MemoryService interface {
	// AddEpisode 对话结束后异步调用：LLM 结构化提取 → 落图。
	// modelID 为当前用户会话实际使用的对话模型（为空或不可用时由工厂内部兜底），
	// 任何失败（Neo4j 不可用 / 提取失败）只记日志返回 error，不阻断主流程。
	AddEpisode(ctx context.Context, userID, sessionID uint, modelID, query, answer string) error
	// RetrieveMemory 问答前同步调用：LLM 关键词提取 → 图谱检索 → MemoryContext。
	// modelID 为当前用户会话实际使用的对话模型，Neo4j 不可用或检索失败时返回空 Context（nil，调用方跳过注入）。
	RetrieveMemory(ctx context.Context, userID uint, modelID, query string) (*MemoryContext, error)
}
