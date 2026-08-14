package response

// SystemInfoResponse 系统信息响应（GET /api/v1/system/info）
// 字段与前端 web/src/api/system/index.ts 的 SystemInfo 接口一一对应
type SystemInfoResponse struct {
	// Version 应用版本号（ldflags 注入，未注入时回退 config.app.version）
	Version string `json:"version"`
	// Edition 版本形态：standard | lite（前端据此显示徽标并切换精简模式）
	Edition string `json:"edition,omitempty"`
	// CommitID Git 提交短哈希（ldflags 注入，未注入为空）
	CommitID string `json:"commit_id,omitempty"`
	// BuildTime 构建时间（ldflags 注入，未注入为空）
	BuildTime string `json:"build_time,omitempty"`
	// GoVersion Go 运行时版本
	GoVersion string `json:"go_version,omitempty"`
	// KeywordIndexEngine 关键字检索引擎（当前 pg_search/ParadeDB）
	KeywordIndexEngine string `json:"keyword_index_engine,omitempty"`
	// VectorStoreEngine 向量存储引擎（当前 pgvector）
	VectorStoreEngine string `json:"vector_store_engine,omitempty"`
	// GraphDatabaseEngine 图数据库引擎（Neo4j；未启用时为 "Not Enabled"，
	// 前端 GraphSettings 据此判定图数据库能力是否可用）
	GraphDatabaseEngine string `json:"graph_database_engine,omitempty"`
	// MinioEnabled MinIO 对象存储是否已配置
	MinioEnabled bool `json:"minio_enabled,omitempty"`
	// DBVersion PostgreSQL 版本（SELECT version()，查询失败降级为 "unknown"）
	DBVersion string `json:"db_version,omitempty"`
	// DBMigrationError 启动时数据库迁移失败信息（非空时前端展示排障横幅）
	DBMigrationError string `json:"db_migration_error,omitempty"`
	// StartedAt 进程启动时间（RFC3339, UTC）
	StartedAt string `json:"started_at,omitempty"`
	// UptimeSeconds 距进程启动的秒数
	UptimeSeconds int64 `json:"uptime_seconds,omitempty"`
}
