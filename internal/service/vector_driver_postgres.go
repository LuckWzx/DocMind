package service

import (
	"context"
	"fmt"
	"strings"

	"docmind/internal/model/entity"
	"docmind/internal/pipeline"

	pgvector "github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresVectorDriver struct {
	db    *gorm.DB
	store *entity.VectorStore
}

func newPostgresVectorDriver(db *gorm.DB, store *entity.VectorStore) *postgresVectorDriver {
	return &postgresVectorDriver{
		db:    db,
		store: store,
	}
}

func (d *postgresVectorDriver) UpsertChunks(ctx context.Context, items []VectorItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := d.ensureSchema(ctx); err != nil {
		return err
	}

	records := make([]*entity.ChunkVector, 0, len(items))
	for _, item := range items {
		records = append(records, &entity.ChunkVector{
			UserID:          item.UserID,
			VectorStoreID:   item.VectorStoreID,
			KnowledgeBaseID: item.KnowledgeBaseID,
			KnowledgeID:     item.KnowledgeID,
			ChunkID:         item.ChunkID,
			Embedding:       pgvector.NewVector(item.Embedding),
			ContentHash:     item.ContentHash,
			IsEnabled:       true,
		})
	}

	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "chunk_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id",
				"vector_store_id",
				"knowledge_base_id",
				"knowledge_id",
				"embedding",
				"content_hash",
				"is_enabled",
				"updated_at",
			}),
		}).
		Create(&records).Error
}

func (d *postgresVectorDriver) DeleteByChunkIDs(ctx context.Context, chunkIDs []uint) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	return d.db.WithContext(ctx).Where("chunk_id IN ?", chunkIDs).Delete(&entity.ChunkVector{}).Error
}

func (d *postgresVectorDriver) Search(ctx context.Context, params VectorSearchParams) ([]VectorSearchResult, error) {
	if len(params.QueryVector) == 0 {
		return nil, nil
	}
	if err := d.ensureSchema(ctx); err != nil {
		return nil, err
	}

	queryVector := pgvector.NewVector(params.QueryVector)
	orderSQL, scoreSQL, thresholdSQL := buildMetricSQL(parseMetricType(d.store))

	if params.TopK <= 0 {
		params.TopK = 5
	}

	query := d.db.WithContext(ctx).
		Table("chunk_vectors AS cv").
		Select("cv.chunk_id, cv.knowledge_id, cv.knowledge_base_id, c.content, k.title AS knowledge_title, "+scoreSQL+" AS score", queryVector).
		Joins("LEFT JOIN chunks AS c ON cv.chunk_id = c.id AND c.deleted_at IS NULL").
		Joins("LEFT JOIN knowledges AS k ON cv.knowledge_id = k.id AND k.deleted_at IS NULL").
		Where("cv.user_id = ? AND cv.vector_store_id = ? AND cv.is_enabled = ? AND cv.deleted_at IS NULL", params.UserID, params.VectorStoreID, true)

	if len(params.KnowledgeBaseIDs) > 0 {
		query = query.Where("cv.knowledge_base_id IN ?", params.KnowledgeBaseIDs)
	}
	if len(params.KnowledgeIDs) > 0 {
		query = query.Where("cv.knowledge_id IN ?", params.KnowledgeIDs)
	}
	if len(params.ExcludeChunkIDs) > 0 {
		query = query.Where("cv.chunk_id NOT IN ?", params.ExcludeChunkIDs)
	}
	if params.Threshold > 0 {
		query = query.Where(thresholdSQL, queryVector, params.Threshold)
	}

	var results []VectorSearchResult
	// 注意：gorm 的 Order() 仅支持 clause.OrderBy / clause.OrderByColumn / string，
	// 直接传 clause.Expr 会被静默忽略导致不生成 ORDER BY（结果退化为物理顺序）。
	// 必须包一层 clause.OrderBy{Expression: ...} 才能正确生成带参数绑定的排序子句。
	err := query.
		Order(clause.OrderBy{
			Expression: clause.Expr{SQL: orderSQL, Vars: []interface{}{queryVector}},
		}).
		Limit(params.TopK).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

// EnsureSchema 导出方法，供 pipeline 调用
func (d *postgresVectorDriver) EnsureSchema(ctx context.Context) error {
	return d.ensureSchema(ctx)
}

// pipelineVectorDriverAdapter 适配 pipeline.PipelineVectorDriver 接口
type pipelineVectorDriverAdapter struct {
	inner *postgresVectorDriver
}

func (a *pipelineVectorDriverAdapter) EnsureSchema(ctx context.Context) error {
	return a.inner.EnsureSchema(ctx)
}

func (a *pipelineVectorDriverAdapter) Search(ctx context.Context, params pipeline.PipelineVectorSearchParams) ([]pipeline.PipelineVectorSearchResult, error) {
	return a.inner.SearchWithPipelineParams(ctx, params)
}

// SearchWithPipelineParams 适配 pipeline 的检索参数格式
func (d *postgresVectorDriver) SearchWithPipelineParams(ctx context.Context, params pipeline.PipelineVectorSearchParams) ([]pipeline.PipelineVectorSearchResult, error) {
	rawParams := VectorSearchParams{
		UserID:           params.UserID,
		VectorStoreID:    params.VectorStoreID,
		KnowledgeBaseIDs: params.KnowledgeBaseIDs,
		QueryVector:      params.QueryVector,
		TopK:             params.TopK,
		Threshold:        params.Threshold,
	}
	results, err := d.Search(ctx, rawParams)
	if err != nil {
		return nil, err
	}
	pipelineResults := make([]pipeline.PipelineVectorSearchResult, 0, len(results))
	for _, r := range results {
		pipelineResults = append(pipelineResults, pipeline.PipelineVectorSearchResult{
			ChunkID:        r.ChunkID,
			KnowledgeID:    r.KnowledgeID,
			Content:        r.Content,
			KnowledgeTitle: r.KnowledgeTitle,
			Score:          r.Score,
		})
	}
	return pipelineResults, nil
}

func (d *postgresVectorDriver) ensureSchema(ctx context.Context) error {
	if err := d.db.WithContext(ctx).Exec("CREATE EXTENSION IF NOT EXISTS vector").Error; err != nil {
		return fmt.Errorf("启用 pgvector 扩展失败: %w", err)
	}
	if err := d.db.WithContext(ctx).AutoMigrate(&entity.ChunkVector{}); err != nil {
		return fmt.Errorf("迁移 chunk_vectors 表失败: %w", err)
	}
	if err := d.db.WithContext(ctx).Exec(`
ALTER TABLE chunk_vectors
ALTER COLUMN embedding TYPE vector
USING NULLIF(BTRIM(embedding::text), '')::vector
`).Error; err != nil {
		return fmt.Errorf("修正 embedding 列类型失败: %w", err)
	}
	return nil
}

func parseMetricType(store *entity.VectorStore) string {
	config := entity.IndexConfig{}
	_ = parseEntityJSON(store.IndexConfig, &config)
	metric := strings.TrimSpace(strings.ToLower(config.MetricType))
	if metric == "" {
		return "cosine"
	}
	return metric
}

func buildMetricSQL(metric string) (orderSQL, scoreSQL, thresholdSQL string) {
	switch metric {
	case "l2":
		return "cv.embedding <-> ?", "(cv.embedding <-> ?) * -1", "cv.embedding <-> ? <= ?"
	case "ip", "inner_product":
		return "cv.embedding <#> ?", "(cv.embedding <#> ?) * -1", "(cv.embedding <#> ?) * -1 >= ?"
	default:
		return "cv.embedding <=> ?", "1 - (cv.embedding <=> ?)", "1 - (cv.embedding <=> ?) >= ?"
	}
}
