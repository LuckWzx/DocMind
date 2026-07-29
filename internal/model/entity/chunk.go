package entity

import (
	"encoding/json"
	"strings"
)

const (
	// ChunkTypeText 普通文本分块
	ChunkTypeText = "text"
	// ChunkTypeMarkdown Markdown 分块
	ChunkTypeMarkdown = "markdown"
	// ChunkTypeMarkdownParent Markdown 父块，主要用于回溯更大上下文
	ChunkTypeMarkdownParent = "markdown_parent"
	// ChunkTypeMarkdownChild Markdown 子块，主要用于向量检索
	ChunkTypeMarkdownChild = "markdown_child"
)

// ============================================================================
// Chunk 配置依赖
// ============================================================================

// FAQChunkMetadata FAQ 条目的元数据，存入 Chunk.Metadata 列
type FAQChunkMetadata struct {
	StandardQuestion  string   `json:"standard_question"`
	SimilarQuestions  []string `json:"similar_questions,omitempty"`
	NegativeQuestions []string `json:"negative_questions,omitempty"`
	Answers           []string `json:"answers,omitempty"`
	AnswerStrategy    string   `json:"answer_strategy,omitempty"` // all / random
	Version           int      `json:"version,omitempty"`
	Source            string   `json:"source,omitempty"` // import / manual
}

// ChunkMetadata 文档分块的结构化元数据，优先描述 Markdown 上下文
type ChunkMetadata struct {
	DocTitle         string   `json:"doc_title,omitempty"`
	HeadingPath      []string `json:"heading_path,omitempty"`
	ContextHeader    string   `json:"context_header,omitempty"`
	SourceFormat     string   `json:"source_format,omitempty"`
	SourceParser     string   `json:"source_parser,omitempty"`
	EmbeddingVersion string   `json:"embedding_version,omitempty"`
	IsParent         bool     `json:"is_parent,omitempty"`
	ParentLocalIndex int      `json:"parent_local_index,omitempty"`
}

// ContextSummary 返回适合参与 embedding 的上下文标题
func (m ChunkMetadata) ContextSummary() string {
	if header := strings.TrimSpace(m.ContextHeader); header != "" {
		return header
	}

	segments := make([]string, 0, len(m.HeadingPath))
	for _, item := range m.HeadingPath {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return strings.Join(segments, " / ")
}

// Chunk 文本分块，知识被拆分后的最小检索单元
type Chunk struct {
	BaseEntity
	Content         string `gorm:"type:text;comment:分块文本内容，默认保存 Markdown 片段" json:"content"`
	ChunkIndex      int    `gorm:"index;comment:文档中的顺序号" json:"chunk_index"`
	KnowledgeID     uint   `gorm:"index;comment:所属知识条目ID" json:"knowledge_id"`
	KnowledgeBaseID uint   `gorm:"index;comment:所属知识库ID" json:"knowledge_base_id"`
	ChunkType       string `gorm:"type:varchar(32);default:'text';comment:分块类型 text/markdown" json:"chunk_type"`
	ChunkStatus     int    `gorm:"default:0;comment:分块状态 0=默认 1=已存储 2=已索引" json:"chunk_status"`
	ParentChunkID   uint   `gorm:"index;comment:父分块ID" json:"parent_chunk_id"`
	TagID           uint   `gorm:"index;comment:标签ID" json:"tag_id"`
	Metadata        JSON   `gorm:"type:json;comment:扩展元数据，例如标题路径和来源格式" json:"metadata"`
	ContentHash     string `gorm:"type:varchar(64);comment:内容hash 去重/增量更新" json:"content_hash"`
	IsEnabled       bool   `gorm:"default:true;comment:是否启用" json:"is_enabled"`
}

// TableName 指定表名
func (Chunk) TableName() string {
	return "chunks"
}

// MetadataStruct 解析 Chunk.Metadata；解析失败时返回空元数据，避免索引流程被脏数据阻塞
func (c Chunk) MetadataStruct() ChunkMetadata {
	if len(c.Metadata) == 0 {
		return ChunkMetadata{}
	}

	var metadata ChunkMetadata
	if err := json.Unmarshal(c.Metadata, &metadata); err != nil {
		return ChunkMetadata{}
	}
	return metadata
}

// EmbeddingContent 返回给 embedding 模型的输入文本
func (c Chunk) EmbeddingContent(documentTitle string) string {
	metadata := c.MetadataStruct()
	title := strings.TrimSpace(documentTitle)
	if title == "" {
		title = strings.TrimSpace(metadata.DocTitle)
	}

	parts := make([]string, 0, 3)
	if title != "" {
		parts = append(parts, title)
	}
	if contextHeader := metadata.ContextSummary(); contextHeader != "" {
		parts = append(parts, contextHeader)
	}
	if body := strings.TrimSpace(c.Content); body != "" {
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}
