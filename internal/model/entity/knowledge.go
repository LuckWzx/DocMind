package entity

const (
	KnowledgeTypeFile      = "file"
	KnowledgeTypeFAQImport = "faq_import"
)

const (
	KnowledgeParseStatusPending    = "pending"
	KnowledgeParseStatusProcessing = "processing"
	KnowledgeParseStatusFinalizing = "finalizing"
	KnowledgeParseStatusCompleted  = "completed"
	KnowledgeParseStatusFailed     = "failed"
)

// Knowledge 知识条目，知识库中的一条知识（一份文档或一组 FAQ）
type Knowledge struct {
	BaseEntity
	Title           string `gorm:"type:varchar(255);comment:条目标题" json:"title"`
	FileName        string `gorm:"type:varchar(255);comment:文件名" json:"file_name"`
	Description     string `gorm:"type:text;comment:条目描述" json:"description"`
	Type            string `gorm:"type:varchar(32);default:'file';comment:类型 file/faq_import" json:"type"`
	Source          string `gorm:"type:varchar(32);default:'web';comment:来源 web/url/manual/api" json:"source"`
	Channel         string `gorm:"type:varchar(32);comment:接入渠道" json:"channel"`
	ParseStatus     string `gorm:"type:varchar(32);default:'pending';comment:解析状态" json:"parse_status"`
	SummaryStatus   string `gorm:"type:varchar(32);default:'completed';comment:摘要状态" json:"summary_status"`
	ProcessingStage string `gorm:"type:varchar(32);default:'queued';comment:处理阶段 queued/parsing/indexing/done/failed" json:"processing_stage"`
	KnowledgeBaseID uint   `gorm:"index;comment:所属知识库ID" json:"knowledge_base_id"`
	FileURL         string `gorm:"type:text;comment:原始文件或源数据定位" json:"file_url"`
	FileType        string `gorm:"type:varchar(32);comment:文件类型" json:"file_type"`
	FileSize        int64  `gorm:"comment:文件大小" json:"file_size"`
	TagID           uint   `gorm:"index;comment:标签ID" json:"tag_id"`
	ProcessConfig   JSON   `gorm:"type:json;comment:单次处理配置" json:"process_config"`
	ErrorMessage    string `gorm:"type:text;comment:错误信息" json:"error_message"`
}

// TableName 指定表名
func (Knowledge) TableName() string {
	return "knowledges"
}
