package response

import (
	"time"

	"docmind/internal/model/entity"
)

type KnowledgeTagLite struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type KnowledgeResponse struct {
	ID              uint               `json:"id"`
	KnowledgeBaseID uint               `json:"knowledge_base_id"`
	Title           string             `json:"title"`
	FileName        string             `json:"file_name"`
	Description     string             `json:"description"`
	Type            string             `json:"type"`
	Source          string             `json:"source"`
	Channel         string             `json:"channel"`
	ParseStatus     string             `json:"parse_status"`
	SummaryStatus   string             `json:"summary_status"`
	FileType        string             `json:"file_type"`
	FileSize        int64              `json:"file_size"`
	TagID           uint               `json:"tag_id"`
	Tags            []KnowledgeTagLite `json:"tags,omitempty"`
	ErrorMessage    string             `json:"error_message"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type KnowledgeDetailResponse struct {
	ID              uint               `json:"id"`
	KnowledgeBaseID uint               `json:"knowledge_base_id"`
	Title           string             `json:"title"`
	FileName        string             `json:"file_name"`
	Description     string             `json:"description"`
	Type            string             `json:"type"`
	Source          string             `json:"source"`
	Channel         string             `json:"channel"`
	ParseStatus     string             `json:"parse_status"`
	SummaryStatus   string             `json:"summary_status"`
	ProcessingStage string             `json:"processing_stage"`
	FileType        string             `json:"file_type"`
	FileSize        int64              `json:"file_size"`
	FileURL         string             `json:"file_url,omitempty"`
	FilePath        string             `json:"file_path,omitempty"`
	TagID           uint               `json:"tag_id"`
	Tags            []KnowledgeTagLite `json:"tags,omitempty"`
	ProcessConfig   entity.JSON        `json:"process_config,omitempty"`
	ErrorMessage    string             `json:"error_message"`
	ChunkCount      int64              `json:"chunk_count"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// KnowledgeFileUploadResponse 文件上传并解析响应
type KnowledgeFileUploadResponse struct {
	KnowledgeID     uint   `json:"knowledge_id"`
	KnowledgeBaseID uint   `json:"knowledge_base_id"`
	Title           string `json:"title"`
	FileType        string `json:"file_type"`
	FilePath        string `json:"file_path"`
	ParseStatus     string `json:"parse_status"`
	ChunkCount      int    `json:"chunk_count"`
	MarkdownChars   int    `json:"markdown_chars"`
}

// SpanNode 处理流水线追踪节点
// 对应前端 knowledge-processing-timeline.vue 的 SpanNode 接口
type SpanNode struct {
	SpanID       string      `json:"span_id,omitempty"`
	ParentSpanID string      `json:"parent_span_id,omitempty"`
	Name         string      `json:"name"`
	Kind         string      `json:"kind"`
	Status       string      `json:"status"`
	StartedAt    *string     `json:"started_at"`
	FinishedAt   *string     `json:"finished_at"`
	DurationMs   int64       `json:"duration_ms,omitempty"`
	ErrorCode    string      `json:"error_code,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	Children     []*SpanNode `json:"children,omitempty"`
}

// KnowledgeSpansResponse 处理时间线响应
// 对应前端 SpansResponse 接口
type KnowledgeSpansResponse struct {
	KnowledgeID   uint      `json:"knowledge_id"`
	Attempt       int       `json:"attempt"`
	LatestAttempt int       `json:"latest_attempt"`
	ParseStatus   string    `json:"parse_status"`
	CurrentStage  string    `json:"current_stage,omitempty"`
	Trace         *SpanNode `json:"trace"`
	LastError     *SpanNode `json:"last_error,omitempty"`
}
