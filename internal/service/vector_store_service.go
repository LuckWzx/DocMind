package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	"docmind/pkg/config"
	bizerrors "docmind/pkg/errors"
	pkgresponse "docmind/pkg/response"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const chunkStatusIndexed = 2

// vectorStoreService 向量存储服务实现
type vectorStoreService struct {
	vectorStoreRepo   repository.VectorStoreRepository
	knowledgeBaseRepo repository.KnowledgeBaseRepository
	chunkRepo         repository.ChunkRepository
	modelService      ModelService
	primaryDB         *gorm.DB
	appConfig         *config.Config
}

// NewVectorStoreService 创建向量存储服务
func NewVectorStoreService(
	vectorStoreRepo repository.VectorStoreRepository,
	knowledgeBaseRepo repository.KnowledgeBaseRepository,
	chunkRepo repository.ChunkRepository,
	modelService ModelService,
	primaryDB *gorm.DB,
	appConfig *config.Config,
) VectorStoreService {
	return &vectorStoreService{
		vectorStoreRepo:   vectorStoreRepo,
		knowledgeBaseRepo: knowledgeBaseRepo,
		chunkRepo:         chunkRepo,
		modelService:      modelService,
		primaryDB:         primaryDB,
		appConfig:         appConfig,
	}
}

// Create 创建向量存储
func (s *vectorStoreService) Create(userID uint, req *request.CreateVectorStoreRequest) (*dto.VectorStoreResponse, error) {
	connectionConfig, err := normalizeJSON(req.ConnectionConfig)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "connection_config 必须是合法 JSON", err)
	}
	indexConfig, err := normalizeJSON(req.IndexConfig)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "index_config 必须是合法 JSON", err)
	}

	store := &entity.VectorStore{
		UserID:           userID,
		Name:             req.Name,
		EngineType:       req.EngineType,
		ConnectionConfig: connectionConfig,
		IndexConfig:      indexConfig,
		Status:           entity.VectorStoreStatusActive,
	}
	if err := s.vectorStoreRepo.Create(store); err != nil {
		return nil, err
	}
	return toVectorStoreResponse(store), nil
}

// GetByID 获取向量存储详情
func (s *vectorStoreService) GetByID(userID, id uint) (*dto.VectorStoreResponse, error) {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return nil, err
	}
	return toVectorStoreResponse(store), nil
}

// Update 更新向量存储
func (s *vectorStoreService) Update(userID, id uint, req *request.UpdateVectorStoreRequest) (*dto.VectorStoreResponse, error) {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		store.Name = req.Name
	}
	if req.EngineType != "" {
		store.EngineType = req.EngineType
	}
	if req.ConnectionConfig != "" {
		connectionConfig, err := normalizeJSON(req.ConnectionConfig)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "connection_config 必须是合法 JSON", err)
		}
		store.ConnectionConfig = connectionConfig
	}
	if req.IndexConfig != "" {
		indexConfig, err := normalizeJSON(req.IndexConfig)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "index_config 必须是合法 JSON", err)
		}
		store.IndexConfig = indexConfig
	}
	if req.Status != nil {
		store.Status = *req.Status
	}

	if err := s.vectorStoreRepo.Update(store); err != nil {
		return nil, err
	}
	return toVectorStoreResponse(store), nil
}

// Delete 删除向量存储
func (s *vectorStoreService) Delete(userID, id uint) error {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return err
	}
	return s.vectorStoreRepo.Delete(store.ID)
}

// List 分页获取向量存储
func (s *vectorStoreService) List(userID uint, req *request.VectorStoreListRequest) (*pkgresponse.PageResponse, error) {
	offset := (req.Page - 1) * req.Size
	stores, total, err := s.vectorStoreRepo.ListByUser(userID, offset, req.Size)
	if err != nil {
		return nil, err
	}

	list := make([]*dto.VectorStoreResponse, 0, len(stores))
	for _, store := range stores {
		list = append(list, toVectorStoreResponse(store))
	}

	return pkgresponse.NewPageResponse(list, total, req.Page, req.Size), nil
}

// TestConnection 测试向量存储连接
func (s *vectorStoreService) TestConnection(userID, id uint) error {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return err
	}

	driver, cleanup, err := s.getDriver(store)
	if err != nil {
		return err
	}
	defer cleanup()

	switch typed := driver.(type) {
	case *postgresVectorDriver:
		return typed.ensureSchema(context.Background())
	default:
		return bizerrors.New(bizerrors.CodeNotImplemented, "暂不支持该向量引擎的连接测试")
	}
}

// Search 执行向量检索
func (s *vectorStoreService) Search(ctx context.Context, userID, id uint, req *request.VectorSearchRequest) ([]*dto.VectorSearchResultResponse, error) {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return nil, err
	}

	driver, cleanup, err := s.getDriver(store)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	vector, err := s.modelService.EmbedText(userID, req.EmbeddingModelID, req.Query)
	if err != nil {
		return nil, err
	}

	results, err := driver.Search(ctx, VectorSearchParams{
		UserID:           userID,
		VectorStoreID:    store.ID,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		KnowledgeIDs:     req.KnowledgeIDs,
		QueryVector:      vector,
		TopK:             req.TopK,
		Threshold:        req.Threshold,
		ExcludeChunkIDs:  req.ExcludeChunkIDs,
	})
	if err != nil {
		return nil, err
	}

	contentByChunkID, err := s.loadChunkContentMap(extractChunkIDs(results))
	if err != nil {
		return nil, err
	}

	resp := make([]*dto.VectorSearchResultResponse, 0, len(results))
	for _, item := range results {
		resp = append(resp, &dto.VectorSearchResultResponse{
			ChunkID:         item.ChunkID,
			KnowledgeID:     item.KnowledgeID,
			KnowledgeBaseID: item.KnowledgeBaseID,
			Content:         contentByChunkID[item.ChunkID],
			Score:           item.Score,
		})
	}
	return resp, nil
}

// IndexKnowledgeBase 将知识库分块写入向量索引
func (s *vectorStoreService) IndexKnowledgeBase(ctx context.Context, userID, id, knowledgeBaseID uint, req *request.IndexKnowledgeBaseRequest) (*dto.IndexKnowledgeBaseResponse, error) {
	store, err := s.GetEntityByID(userID, id)
	if err != nil {
		return nil, err
	}

	kb, err := s.knowledgeBaseRepo.FindByID(knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	if kb.VectorStoreID != nil && *kb.VectorStoreID != id {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "知识库未绑定当前向量存储")
	}

	modelRef := strings.TrimSpace(kb.EmbeddingModelID)
	if modelRef == "" {
		modelRef = strings.TrimSpace(kb.IndexingStrategy.EmbeddingModel)
	}
	if modelRef == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "知识库未配置 embedding 模型")
	}

	chunks, err := s.chunkRepo.ListByKnowledgeBase(knowledgeBaseID, req.ChunkIDs)
	if err != nil {
		return nil, err
	}
	knowledgeTitleMap, err := s.loadKnowledgeTitleMap(chunks)
	if err != nil {
		return nil, err
	}

	items := make([]VectorItem, 0, len(chunks))
	indexedChunkIDs := make([]uint, 0, len(chunks))
	for _, chunk := range chunks {
		embeddingContent := chunk.EmbeddingContent(knowledgeTitleMap[chunk.KnowledgeID])
		if strings.TrimSpace(embeddingContent) == "" {
			continue
		}

		vector, err := s.modelService.EmbedText(userID, modelRef, embeddingContent)
		if err != nil {
			return nil, err
		}

		items = append(items, VectorItem{
			UserID:          userID,
			VectorStoreID:   store.ID,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			KnowledgeID:     chunk.KnowledgeID,
			ChunkID:         chunk.ID,
			Embedding:       vector,
			ContentHash:     chunk.ContentHash,
		})
		indexedChunkIDs = append(indexedChunkIDs, chunk.ID)
	}

	driver, cleanup, err := s.getDriver(store)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if err := driver.UpsertChunks(ctx, items); err != nil {
		return nil, err
	}
	if err := s.chunkRepo.UpdateStatusByIDs(indexedChunkIDs, chunkStatusIndexed); err != nil {
		return nil, err
	}

	return &dto.IndexKnowledgeBaseResponse{
		KnowledgeBaseID: knowledgeBaseID,
		IndexedCount:    len(indexedChunkIDs),
	}, nil
}

// GetEntityByID 获取向量存储实体
func (s *vectorStoreService) GetEntityByID(userID, id uint) (*entity.VectorStore, error) {
	store, err := s.vectorStoreRepo.FindByUserAndID(userID, id)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	return store, nil
}

func (s *vectorStoreService) getDriver(store *entity.VectorStore) (VectorDriver, func(), error) {
	switch strings.ToLower(store.EngineType) {
	case entity.VectorStoreEnginePostgres:
		db, cleanup, err := s.resolvePostgresDB(store)
		if err != nil {
			return nil, nil, err
		}
		return newPostgresVectorDriver(db, store), cleanup, nil
	default:
		return nil, func() {}, bizerrors.New(bizerrors.CodeNotImplemented, "当前仅实现 postgres 向量引擎")
	}
}

func (s *vectorStoreService) resolvePostgresDB(store *entity.VectorStore) (*gorm.DB, func(), error) {
	connCfg := entity.ConnectionConfig{}
	if err := parseEntityJSON(store.ConnectionConfig, &connCfg); err != nil {
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "解析 connection_config 失败", err)
	}

	if connCfg.UseDefaultConnection || strings.TrimSpace(connCfg.Host) == "" {
		return s.primaryDB, func() {}, nil
	}

	dsn := buildPostgresDSN(connCfg, s.appConfig)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
		Logger:                                   gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "连接自定义 PostgreSQL 失败", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "获取自定义 PostgreSQL 连接失败", err)
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "自定义 PostgreSQL Ping 失败", err)
	}

	return db, func() {
		_ = sqlDB.Close()
	}, nil
}

func buildPostgresDSN(connCfg entity.ConnectionConfig, appCfg *config.Config) string {
	host := connCfg.Host
	port := connCfg.Port
	username := connCfg.Username
	password := connCfg.Password
	databaseName := connCfg.Database
	sslMode := strings.TrimSpace(connCfg.SSLMode)

	if appCfg != nil {
		if host == "" {
			host = appCfg.Database.PostgreSQL.Host
		}
		if port == 0 {
			port = appCfg.Database.PostgreSQL.Port
		}
		if username == "" {
			username = appCfg.Database.PostgreSQL.Username
		}
		if password == "" {
			password = appCfg.Database.PostgreSQL.Password
		}
		if databaseName == "" {
			databaseName = appCfg.Database.PostgreSQL.Database
		}
		if sslMode == "" {
			sslMode = appCfg.Database.PostgreSQL.SSLMode
		}
	}
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		host,
		username,
		password,
		databaseName,
		port,
		sslMode,
	)
}

func loadResponse(store *entity.VectorStore) *dto.VectorStoreResponse {
	return toVectorStoreResponse(store)
}

func toVectorStoreResponse(store *entity.VectorStore) *dto.VectorStoreResponse {
	return &dto.VectorStoreResponse{
		ID:               store.ID,
		UserID:           store.UserID,
		Name:             store.Name,
		EngineType:       store.EngineType,
		ConnectionConfig: jsonString(store.ConnectionConfig),
		IndexConfig:      jsonString(store.IndexConfig),
		Status:           store.Status,
		CreatedAt:        store.CreatedAt,
		UpdatedAt:        store.UpdatedAt,
	}
}

func normalizeJSON(raw string) (entity.JSON, error) {
	if raw == "" {
		return nil, nil
	}

	var payload json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return entity.JSON(normalized), nil
}

func parseEntityJSON[T any](raw entity.JSON, target *T) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func jsonString(raw entity.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	return string(raw)
}

func extractChunkIDs(items []VectorSearchResult) []uint {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ChunkID)
	}
	return ids
}

func (s *vectorStoreService) loadChunkContentMap(chunkIDs []uint) (map[uint]string, error) {
	result := make(map[uint]string, len(chunkIDs))
	if len(chunkIDs) == 0 {
		return result, nil
	}

	chunks, err := s.chunkRepo.ListByIDs(chunkIDs)
	if err != nil {
		return nil, err
	}
	for _, chunk := range chunks {
		result[chunk.ID] = chunk.Content
	}
	return result, nil
}

func (s *vectorStoreService) loadKnowledgeTitleMap(chunks []*entity.Chunk) (map[uint]string, error) {
	result := make(map[uint]string)
	knowledgeIDs := make([]uint, 0, len(chunks))
	seen := make(map[uint]struct{}, len(chunks))

	for _, chunk := range chunks {
		if chunk == nil || chunk.KnowledgeID == 0 {
			continue
		}
		if _, exists := seen[chunk.KnowledgeID]; exists {
			continue
		}
		seen[chunk.KnowledgeID] = struct{}{}
		knowledgeIDs = append(knowledgeIDs, chunk.KnowledgeID)
	}
	if len(knowledgeIDs) == 0 {
		return result, nil
	}

	var records []entity.Knowledge
	if err := s.primaryDB.
		Model(&entity.Knowledge{}).
		Select("id", "title").
		Where("id IN ?", knowledgeIDs).
		Find(&records).Error; err != nil {
		return nil, err
	}

	for _, knowledge := range records {
		result[knowledge.ID] = knowledge.Title
	}
	return result, nil
}
