package service

import (
	"context"
	"runtime"
	"strings"
	"time"

	dto "docmind/internal/model/dto/response"
	"docmind/pkg/config"

	"gorm.io/gorm"
)

// 当前部署形态下的引擎常量（未来如需多引擎可配置化）
const (
	keywordIndexEngineName = "pg_search (ParadeDB)" // BM25 关键字检索（pg_search 扩展）
	vectorStoreEngineName  = "pgvector"             // 向量检索
	graphEngineName        = "Neo4j"                // 图数据库（长期记忆）
	graphEngineDisabled    = "Not Enabled"
	unknownEngine          = "unknown"

	// dbVersionQueryTimeout SELECT version() 查询超时（DB 故障时避免拖慢系统信息页）
	dbVersionQueryTimeout = 3 * time.Second
)

// systemService 系统信息业务实现
type systemService struct {
	pgDB           *gorm.DB
	cfg            *config.Config
	version        string // ldflags 注入的版本（scripts/build.sh），空/unknown 时回退 config.app.version
	commitID       string // ldflags 注入的 Git 提交短哈希
	buildTime      string // ldflags 注入的构建时间
	startedAt      time.Time
	dbMigrationErr string // 启动时 AutoMigrate 失败信息（非空时前端展示排障横幅）
}

// NewSystemService 创建系统信息服务
func NewSystemService(pgDB *gorm.DB, cfg *config.Config, version, commitID, buildTime string, startedAt time.Time, dbMigrationErr string) SystemService {
	return &systemService{
		pgDB:           pgDB,
		cfg:            cfg,
		version:        version,
		commitID:       commitID,
		buildTime:      buildTime,
		startedAt:      startedAt,
		dbMigrationErr: dbMigrationErr,
	}
}

// Info 获取系统信息（只读组装，不做任何写操作）
func (s *systemService) Info(ctx context.Context) *dto.SystemInfoResponse {
	version := strings.TrimSpace(s.version)
	if version == "" || version == "unknown" {
		version = s.cfg.App.Version
	}
	edition := strings.TrimSpace(s.cfg.App.Edition)
	if edition == "" {
		edition = "standard"
	}

	info := &dto.SystemInfoResponse{
		Version:             version,
		Edition:             edition,
		CommitID:            s.commitID,
		BuildTime:           s.buildTime,
		GoVersion:           runtime.Version(),
		KeywordIndexEngine:  keywordIndexEngineName,
		VectorStoreEngine:   vectorStoreEngineName,
		GraphDatabaseEngine: graphEngineDisabled,
		MinioEnabled:        s.minioEnabled(),
		DBVersion:           s.dbVersion(ctx),
		DBMigrationError:    s.dbMigrationErr,
		StartedAt:           s.startedAt.UTC().Format(time.RFC3339),
		UptimeSeconds:       int64(time.Since(s.startedAt).Seconds()),
	}
	if s.cfg.Neo4j.Enabled {
		info.GraphDatabaseEngine = graphEngineName
	}
	return info
}

// minioEnabled 判断 MinIO 对象存储是否已完整配置（关键字段齐全即视为启用）
func (s *systemService) minioEnabled() bool {
	m := s.cfg.MinIO
	return m.Endpoint != "" && m.AccessKeyID != "" && m.AccessKeySecret != "" && m.BucketName != ""
}

// dbVersion 查询 PostgreSQL 版本（SELECT version()）；DB 不可用时降级为 "unknown"，不阻塞接口
func (s *systemService) dbVersion(ctx context.Context) string {
	if s.pgDB == nil {
		return unknownEngine
	}
	ctx, cancel := context.WithTimeout(ctx, dbVersionQueryTimeout)
	defer cancel()

	var v string
	if err := s.pgDB.WithContext(ctx).Raw("SELECT version()").Row().Scan(&v); err != nil {
		return unknownEngine
	}
	return v
}
