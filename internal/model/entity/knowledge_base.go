package entity

import "database/sql/driver"

// ============================================================================
// KnowledgeBase 配置依赖
// ============================================================================

// ParserEngineRule 文件解析引擎规则
type ParserEngineRule struct {
	FileTypes []string `json:"file_types"`
	Engine    string   `json:"engine"`
}

// ChunkingConfig 分块策略配置
type ChunkingConfig struct {
	ChunkSize                 int                `json:"chunk_size"`
	ChunkOverlap              int                `json:"chunk_overlap"`
	Separators                []string           `json:"separators"`
	ParserEngineRules         []ParserEngineRule `json:"parser_engine_rules,omitempty"`
	EnableParentChild         bool               `json:"enable_parent_child"`
	ParentChunkSize           int                `json:"parent_chunk_size,omitempty"`
	ChildChunkSize            int                `json:"child_chunk_size,omitempty"`
	Strategy                  string             `json:"strategy,omitempty"`
	TokenLimit                int                `json:"token_limit,omitempty"`
	Languages                 []string           `json:"languages,omitempty"`
	TableMetadataInstructions string             `json:"table_metadata_instructions,omitempty"`
	EnableMM                  bool               `json:"enable_mm"` // 是否开启多模态
}

func (c *ChunkingConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

func (c ChunkingConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// ExtractConfig 知识抽取配置
type ExtractConfig struct {
	Enabled bool   `json:"enabled"`
	Prompt  string `json:"prompt,omitempty"`
	ModelID string `json:"model_id,omitempty"`
}

func (c *ExtractConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

func (c ExtractConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// FAQConfig FAQ 知识库配置
type FAQConfig struct {
	FAQQuestionIndexMode string `json:"faq_question_index_mode"` // combined / separate
	FAQIndexMode         string `json:"faq_index_mode"`          // question_only / question_answer
}

func (c *FAQConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

func (c FAQConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// IndexingStrategy 索引策略
type IndexingStrategy struct {
	ChunkSize      int    `json:"chunk_size"`
	ChunkOverlap   int    `json:"chunk_overlap"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
	VectorStore    string `json:"vector_store,omitempty"`
}

func (c *IndexingStrategy) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

func (c IndexingStrategy) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// StorageProviderConfig 存储提供方配置
type StorageProviderConfig struct {
	Provider   string            `json:"provider"` // local / minio / s3
	BucketName string            `json:"bucket_name"`
	Endpoint   string            `json:"endpoint,omitempty"`
	AccessKey  string            `json:"access_key,omitempty"`
	SecretKey  string            `json:"secret_key,omitempty"`
	Region     string            `json:"region,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

func (c *StorageProviderConfig) Scan(value interface{}) error {
	return scanJSONStruct(value, c)
}

func (c StorageProviderConfig) Value() (driver.Value, error) {
	return jsonStructValue(c)
}

// KnowledgeBase 知识库，知识条目的容器
type KnowledgeBase struct {
	BaseEntity
	Name                  string                 `gorm:"type:varchar(255);not null;comment:知识库名称" json:"name"`
	Type                  string                 `gorm:"type:varchar(32);default:'document';comment:类型 document/faq" json:"type"`
	Description           string                 `gorm:"type:text;comment:知识库描述" json:"description"`
	EmbeddingModelID      string                 `gorm:"type:varchar(64);comment:Embedding模型ID" json:"embedding_model_id"`
	SummaryModelID        string                 `gorm:"type:varchar(64);comment:摘要模型ID" json:"summary_model_id"`
	IsPinned              bool                   `gorm:"default:false;comment:是否置顶" json:"is_pinned"`
	ChunkingConfig        ChunkingConfig         `gorm:"type:json;comment:分块配置" json:"chunking_config"`
	ExtractConfig         *ExtractConfig         `gorm:"type:json;comment:抽取配置" json:"extract_config"`
	FAQConfig             *FAQConfig             `gorm:"type:json;comment:FAQ配置" json:"faq_config"`
	StorageProviderConfig *StorageProviderConfig `gorm:"column:storage_provider_config;type:jsonb;comment:存储配置" json:"storage_provider_config"`
	VectorStoreID         *uint                  `gorm:"index;comment:向量存储ID nil=使用默认" json:"vector_store_id"`
	IndexingStrategy      IndexingStrategy       `gorm:"type:json;comment:索引策略" json:"indexing_strategy"`
}

// TableName 指定表名
func (KnowledgeBase) TableName() string {
	return "knowledge_bases"
}
