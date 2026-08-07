package entity

// 会话摘要类型常量
const (
	// SummaryTypeLLM 摘要由 LLM 生成
	SummaryTypeLLM = "llm"
	// SummaryTypeRaw 摘要降级为原文归档（LLM 不可用）
	SummaryTypeRaw = "raw"
)

// SessionSummary 会话短期记忆摘要（增量压缩的持久化状态）。
//
// 每个会话至多一行：记录 LLM 压缩产生的摘要内容，以及压缩边界
// （LastMessageID 之前的消息已并入摘要，后续加载历史时只取边界之后
// 的增量消息，避免每次请求全量重算）。
type SessionSummary struct {
	BaseEntity
	SessionID uint   `gorm:"uniqueIndex;not null;comment:所属会话ID" json:"session_id"`
	Content   string `gorm:"type:text;comment:摘要内容（LLM 生成或原文归档）" json:"content"`
	// SummaryType 摘要类型：llm / raw（LLM 失败降级为原文归档）
	SummaryType string `gorm:"type:varchar(16);default:'llm';comment:摘要类型 llm/raw" json:"summary_type"`
	// LastMessageID 压缩边界：摘要已覆盖的最后一条消息 ID。
	// 下次请求只加载 ID 大于此值的消息作为增量。
	LastMessageID uint `gorm:"not null;comment:压缩边界：摘要覆盖到的最后一条消息ID" json:"last_message_id"`
	// CompressedCount 累计已压缩的消息条数（仅统计用途）
	CompressedCount int `gorm:"default:0;comment:累计已压缩消息条数" json:"compressed_count"`
}
