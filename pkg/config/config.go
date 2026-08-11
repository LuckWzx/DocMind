package config

import "time"

// MCPServicePresetConfig 全局（系统级）MCP 服务预置配置
// 启动时 seed 到 mcp_services 表（user_id=0），所有用户可见但只读
// 适合部署在 DocMind 所在机器上的本地程序（stdio）或公共远程服务（sse/http-streamable）
type MCPServicePresetConfig struct {
	Name          string            `mapstructure:"name"`
	Description   string            `mapstructure:"description"`
	TransportType string            `mapstructure:"transport_type"` // sse / stdio / http-streamable
	URL           string            `mapstructure:"url"`            // 远程传输的服务地址
	Enabled       *bool             `mapstructure:"enabled"`        // 默认 true
	Command       string            `mapstructure:"command"`        // stdio 启动命令（如 node / npx）
	Args          []string          `mapstructure:"args"`           // stdio 命令参数
	Env           []string          `mapstructure:"env"`            // stdio 环境变量（KEY=VALUE）
	Headers       map[string]string `mapstructure:"headers"`        // 自定义请求头
	Timeout       int               `mapstructure:"timeout"`        // 超时（秒），默认 30
}

// Config 应用配置结构体
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Log       LogConfig       `mapstructure:"log"`
	CORS      CORSConfig      `mapstructure:"cors"`
	DocReader DocReaderConfig `mapstructure:"docreader"`
	Storage   StorageConfig   `mapstructure:"storage"`
	MinIO     MinIOConfig     `mapstructure:"minio"`
	SSE       SSEConfig       `mapstructure:"sse"`
	Neo4j     Neo4jConfig     `mapstructure:"neo4j"`
	Memory    MemoryConfig    `mapstructure:"memory"`
	CozeLoop  CozeLoopConfig  `mapstructure:"cozeloop"`
	Retrieval RetrievalConfig `mapstructure:"retrieval"`
	// MCPPresetServices 全局 MCP 服务预置（user_id=0，系统级只读）
	MCPPresetServices []MCPServicePresetConfig `mapstructure:"mcp_preset_services"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Mode    string `mapstructure:"mode"` // debug, release, test
	Port    int    `mapstructure:"port"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	PostgreSQL PostgreSQLConfig `mapstructure:"postgresql"`
	MySQL      MySQLConfig      `mapstructure:"mysql"`
	Redis      RedisConfig      `mapstructure:"redis"`
}

// MySQLConfig MySQL 配置
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

// PostgreSQLConfig PostgreSQL 配置
type PostgreSQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	Database     string `mapstructure:"database"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	SSLMode      string `mapstructure:"sslmode"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret      string        `mapstructure:"secret"`
	ExpireHours time.Duration `mapstructure:"expire_hours"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	Filename   string `mapstructure:"filename"`    // 日志文件路径
	MaxSize    int    `mapstructure:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int    `mapstructure:"max_backups"` // 保留的旧日志文件数量
	MaxAge     int    `mapstructure:"max_age"`     // 保留旧日志文件的最大天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// DocReaderConfig DocReader gRPC 配置
type DocReaderConfig struct {
	Addr string `mapstructure:"addr"`
}

// StorageConfig 本地文件存储配置
type StorageConfig struct {
	LocalRoot string `mapstructure:"local_root"`
}

// SSEConfig SSE 流式连接配置
// Duration 字段在 YAML 中写 "15s" 形式（viper 默认支持字符串转 Duration）
type SSEConfig struct {
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`  // 心跳间隔
	IdempotencyTTL    time.Duration `mapstructure:"idempotency_ttl"`     // 幂等键 TTL
	TotalTimeout      time.Duration `mapstructure:"total_timeout"`       // 单次问答总执行超时
	FirstTokenTimeout time.Duration `mapstructure:"first_token_timeout"` // 首 token 超时
	ShutdownGrace     time.Duration `mapstructure:"shutdown_grace"`      // 优雅关闭时通知活跃连接后的等待
	MaxBodyBytes      int64         `mapstructure:"max_body_bytes"`      // 请求体大小上限（字节）
}

// MinIOConfig MinIO 对象存储配置
type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	BaseURL         string `mapstructure:"base_url"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
	PathPrefix      string `mapstructure:"path_prefix"`
	UseSSL          bool   `mapstructure:"use_ssl"`
}

// Neo4jConfig 图数据库配置（长期记忆存储）
type Neo4jConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	URI      string `mapstructure:"uri"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// MemoryConfig 长期记忆配置
// 与短期记忆（internal/memory 压缩器）不同：这里控制跨会话记忆（Neo4j 知识图谱）
type MemoryConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	ModelID               string `mapstructure:"model_id"`
	RetrieveLimit         int    `mapstructure:"retrieve_limit"`
	MaxEpisodesPerSession int    `mapstructure:"max_episodes_per_session"`
}

// RetrievalConfig 检索配置
// DisableBM25=true 时关闭 BM25 关键字检索（仅剩向量检索一路，应急降载用）
// 零值 false = 不关闭，未配置该段时行为与旧版一致
// 生效方式：修改后重启后端；快速问答与智能推理（kb_search）同时受控
// 注：Agent 级仍可用 keyword_top_k<=0 细粒度关闭（见 pipeline.SearchKB）
type RetrievalConfig struct {
	DisableBM25 bool `mapstructure:"disable_bm25"`
}

// CozeLoopConfig CozeLoop 链路追踪配置（可选，Eino 全局 Trace 上报）
// 同时配置 workspace_id 与 api_token 时启用：全局挂载 callbacks handler 后，
// 进程内所有 Eino 组件（Agent 引擎 / RAG 管道 / ChatModel / Embedder / Reranker）自动上报
// APIBaseURL 为空时使用 SDK 默认国内版 https://api.coze.cn
type CozeLoopConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	WorkspaceID string `mapstructure:"workspace_id"`
	APIToken    string `mapstructure:"api_token"`
	APIBaseURL  string `mapstructure:"api_base_url"`
}
