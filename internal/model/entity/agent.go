package entity

import (
	"database/sql/driver"
)

// AgentConfig 智能体配置（JSON 存储，字段对齐前端 CustomAgentConfig）
type AgentConfig struct {
	// 基础设置
	AgentMode       string `json:"agent_mode"` // quick-answer / smart-reasoning
	SystemPrompt    string `json:"system_prompt"`
	ContextTemplate string `json:"context_template,omitempty"`

	// 模型设置
	ModelID             string   `json:"model_id"`
	RerankModelID       string   `json:"rerank_model_id"`
	Temperature         *float64 `json:"temperature,omitempty"`
	MaxCompletionTokens *int     `json:"max_completion_tokens,omitempty"`
	Thinking            *bool    `json:"thinking,omitempty"`
	CitationEnabled     *bool    `json:"citation_enabled,omitempty"`

	// Agent 模式设置
	MaxIterations     *int     `json:"max_iterations,omitempty"`
	LLMCallTimeout    *int     `json:"llm_call_timeout,omitempty"`
	AllowedTools      []string `json:"allowed_tools,omitempty"`
	ReflectionEnabled *bool    `json:"reflection_enabled,omitempty"`

	// MCP 服务设置
	MCPSelectionMode   string   `json:"mcp_selection_mode,omitempty"`
	MCPServices        []string `json:"mcp_services,omitempty"`
	MCPAuthWaitTimeout *int     `json:"mcp_auth_wait_timeout,omitempty"`

	// Skills 设置
	SkillsSelectionMode string   `json:"skills_selection_mode,omitempty"`
	SelectedSkills      []string `json:"selected_skills,omitempty"`

	// 知识库设置
	KBSelectionMode             string   `json:"kb_selection_mode,omitempty"`
	KnowledgeBases              []string `json:"knowledge_bases"`
	RetrieveKBOnlyWhenMentioned *bool    `json:"retrieve_kb_only_when_mentioned,omitempty"`

	// Agent 类型预设
	AgentType      string `json:"agent_type,omitempty"`
	SystemPromptID string `json:"system_prompt_id,omitempty"`

	// 图片上传/多模态设置
	ImageUploadEnabled            *bool  `json:"image_upload_enabled,omitempty"`
	VLMModelID                    string `json:"vlm_model_id,omitempty"`
	ImageStorageProvider          string `json:"image_storage_provider,omitempty"`
	AudioUploadEnabled            *bool  `json:"audio_upload_enabled,omitempty"`
	ASRModelID                    string `json:"asr_model_id,omitempty"`
	AttachmentImageUnderstanding  *bool  `json:"attachment_image_understanding,omitempty"`
	AttachmentOCRMaxPages         *int   `json:"attachment_ocr_max_pages,omitempty"`
	AttachmentParseWaitTimeoutSec *int   `json:"attachment_parse_wait_timeout_sec,omitempty"`

	// 聊天附件解析引擎策略
	ChatParserEngineRules []interface{} `json:"chat_parser_engine_rules,omitempty"`

	// 文件类型限制
	SupportedFileTypes []string `json:"supported_file_types,omitempty"`

	// 检索策略
	EnableQueryExpansion *bool    `json:"enable_query_expansion,omitempty"`
	EmbeddingTopK        int      `json:"embedding_top_k"`
	KeywordTopK          int      `json:"keyword_top_k,omitempty"`
	KeywordThreshold     *float64 `json:"keyword_threshold,omitempty"`
	VectorThreshold      float64  `json:"vector_threshold"`
	RerankTopK           int      `json:"rerank_top_k"`
	RerankThreshold      *float64 `json:"rerank_threshold,omitempty"`

	// FAQ 策略
	FAQPriorityEnabled       *bool    `json:"faq_priority_enabled,omitempty"`
	FAQDirectAnswerThreshold *float64 `json:"faq_direct_answer_threshold,omitempty"`
	FAQScoreBoost            *float64 `json:"faq_score_boost,omitempty"`

	// 网络搜索设置
	WebSearchEnabled    *bool  `json:"web_search_enabled,omitempty"`
	WebSearchProviderID string `json:"web_search_provider_id,omitempty"`
	WebSearchMaxResults *int   `json:"web_search_max_results,omitempty"`
	WebFetchEnabled     *bool  `json:"web_fetch_enabled,omitempty"`
	WebFetchTopN        *int   `json:"web_fetch_top_n,omitempty"`

	// 多轮对话设置
	MultiTurnEnabled *bool `json:"multi_turn_enabled,omitempty"`
	// HistoryTurns 压缩时保底保留的最近完整轮数：对话过长触发短期记忆压缩时，
	// 最近 N 轮对话以原文保留不压缩（与摘要一起发给模型），更早的历史才被压缩。
	HistoryTurns           int    `json:"history_turns"`
	EnableRewrite          *bool  `json:"enable_rewrite,omitempty"`
	QueryUnderstandModelID string `json:"query_understand_model_id,omitempty"`
	RewritePromptSystem    string `json:"rewrite_prompt_system,omitempty"`
	RewritePromptUser      string `json:"rewrite_prompt_user,omitempty"`

	// 兜底策略
	FallbackStrategy string `json:"fallback_strategy,omitempty"`
	FallbackResponse string `json:"fallback_response,omitempty"`
	FallbackPrompt   string `json:"fallback_prompt,omitempty"`

	// 问题推荐
	QuestionSuggestions interface{} `json:"question_suggestions,omitempty"`

	// 意图提示词
	IntentPrompts interface{} `json:"intent_prompts,omitempty"`

	// 其他
	WelcomeMessage      string `json:"welcome_message,omitempty"`
	DataAnalysisEnabled *bool  `json:"data_analysis_enabled,omitempty"`
}

// Scan 实现 sql.Scanner
func (c *AgentConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

// Value 实现 driver.Valuer
func (c AgentConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// Agent 智能体
type Agent struct {
	BaseEntity
	UserID      uint        `gorm:"index;default:0;not null;comment:所属用户ID(0=内置)" json:"user_id"`
	IDStr       string      `gorm:"type:varchar(128);uniqueIndex;not null;comment:字符串ID" json:"id_str"`
	Name        string      `gorm:"type:varchar(255);not null;comment:名称" json:"name"`
	Description string      `gorm:"type:text;comment:描述" json:"description"`
	Avatar      string      `gorm:"type:varchar(512);comment:头像" json:"avatar"`
	IsBuiltin   bool        `gorm:"default:false;comment:是否内置" json:"is_builtin"`
	Config      AgentConfig `gorm:"type:json;comment:智能体配置" json:"config"`

	// HasOverride 当前用户是否已个性化覆盖该内置智能体（不落库，仅接口返回）
	HasOverride bool `gorm:"-" json:"has_override"`
}

// TableName 指定表名
func (Agent) TableName() string {
	return "agents"
}
