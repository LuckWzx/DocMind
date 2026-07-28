package entity

// VectorStoreEngine 向量存储引擎类型
const (
	VectorStoreEnginePostgres      = "postgres"
	VectorStoreEngineQdrant        = "qdrant"
	VectorStoreEngineMilvus        = "milvus"
	VectorStoreEngineWeaviate      = "weaviate"
	VectorStoreEngineElasticsearch = "elasticsearch"
)

// VectorStoreStatus 向量存储状态
const (
	VectorStoreStatusActive   = 1
	VectorStoreStatusDisabled = 2
)

// ConnectionConfig 向量存储连接配置
type ConnectionConfig struct {
	UseDefaultConnection bool   `json:"use_default_connection"`
	Host                 string `json:"host,omitempty"`
	Port                 int    `json:"port,omitempty"`
	Username             string `json:"username,omitempty"`
	Password             string `json:"password,omitempty"`
	Database             string `json:"database,omitempty"`
	SSLMode              string `json:"ssl_mode,omitempty"`
	UseTLS               bool   `json:"use_tls,omitempty"`
	APIKey               string `json:"api_key,omitempty"`
	GrpcAddress          string `json:"grpc_address,omitempty"`
	Scheme               string `json:"scheme,omitempty"`
}

// IndexConfig 向量索引配置
type IndexConfig struct {
	CollectionName string                 `json:"collection_name"`
	Dimension      int                    `json:"dimension"`
	MetricType     string                 `json:"metric_type"`
	IndexType      string                 `json:"index_type"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
}

// VectorStore 向量存储实例配置
type VectorStore struct {
	BaseEntity
	UserID           uint   `gorm:"column:user_id;not null;index" json:"user_id"`
	Name             string `gorm:"type:varchar(255);not null;column:name" json:"name"`
	EngineType       string `gorm:"type:varchar(50);not null;column:engine_type" json:"engine_type"`
	ConnectionConfig JSON   `gorm:"type:jsonb;column:connection_config;comment:连接配置" json:"connection_config"`
	IndexConfig      JSON   `gorm:"type:jsonb;column:index_config;comment:索引配置" json:"index_config"`
	Status           int    `gorm:"type:smallint;default:1;column:status" json:"status"`
}

// TableName 指定表名
func (VectorStore) TableName() string {
	return "vector_stores"
}
