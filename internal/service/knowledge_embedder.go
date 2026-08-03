package service

import (
	"context"
	"fmt"

	"docmind/internal/model/entity"
	"docmind/pkg/logger"

	pgvector "github.com/pgvector/pgvector-go"
	"gorm.io/gorm/clause"
)

// embedChunks 将分块内容向量化并写入 chunk_vectors 表
// 对于大文件（chunks 数量多/单块内容大），自动分批处理以适配 Embedding API 限制
const (
	embedBatchSize        = 5    // 每批分块数（免费 API 通常有单次 token 上限）
	embedMaxCharsPerChunk = 2000 // 单块最大字符数，超出截断
)

func (s *knowledgeService) embedChunks(ctx context.Context, userID uint, knowledge *entity.Knowledge, chunks []*entity.Chunk, kb *entity.KnowledgeBase) error {
	if s.embedderFactory == nil || s.db == nil {
		logger.Warnf("[embedChunks] embedderFactory 或 db 未初始化，跳过向量化")
		return nil
	}
	if kb.EmbeddingModelID == "" {
		logger.Warnf("[embedChunks] 知识库 %d 未配置 EmbeddingModelID，跳过向量化", kb.ID)
		return nil
	}
	if len(chunks) == 0 {
		return nil
	}

	embedder, err := s.embedderFactory.CreateEmbedder(ctx, kb.EmbeddingModelID)
	if err != nil {
		return fmt.Errorf("创建 Embedder 失败 (modelID=%s): %w", kb.EmbeddingModelID, err)
	}

	// 批量调用 Embedding API，避免单次请求 token 数过大
	total := len(chunks)
	logger.Infof("[embedChunks] knowledge=%d 开始向量化，共 %d 个分块，批次大小 %d，单块截断 %d 字符",
		knowledge.ID, total, embedBatchSize, embedMaxCharsPerChunk)

	for start := 0; start < total; start += embedBatchSize {
		end := start + embedBatchSize
		if end > total {
			end = total
		}
		batch := chunks[start:end]
		batchNum := start/embedBatchSize + 1
		totalBatches := (total + embedBatchSize - 1) / embedBatchSize

		texts := make([]string, len(batch))
		totalChars := 0
		for i, chunk := range batch {
			content := chunk.Content
			// 超长分块截断，避免 token 超限
			if len(content) > embedMaxCharsPerChunk {
				content = content[:embedMaxCharsPerChunk]
			}
			texts[i] = content
			totalChars += len(content)
		}

		logger.Infof("[embedChunks] 批次 %d/%d: %d 个分块，共 %d 字符", batchNum, totalBatches, len(batch), totalChars)

		vectors2D, err := embedder.EmbedStrings(ctx, texts)
		if err != nil {
			return fmt.Errorf("批次 %d/%d 向量化失败: %w", batchNum, totalBatches, err)
		}

		if err := s.persistVectors(userID, knowledge, batch, vectors2D, kb); err != nil {
			return fmt.Errorf("批次 %d/%d 保存向量失败: %w", batchNum, totalBatches, err)
		}

		logger.Infof("[embedChunks] 批次 %d/%d 完成", batchNum, totalBatches)
	}

	logger.Infof("[embedChunks] knowledge=%d 向量化全部完成，共 %d 个分块", knowledge.ID, total)
	return nil
}

// persistVectors 将向量写入 chunk_vectors 表
func (s *knowledgeService) persistVectors(userID uint, knowledge *entity.Knowledge, chunks []*entity.Chunk, vectors2D [][]float64, kb *entity.KnowledgeBase) error {

	vectorStoreID := kb.VectorStoreID
	if vectorStoreID == nil {
		var defaultStore entity.VectorStore
		if err := s.db.Where("status = ?", entity.VectorStoreStatusActive).First(&defaultStore).Error; err != nil {
			logger.Warnf("[embedChunks] 知识库 %d 未配置 VectorStoreID 且未找到可用向量存储，跳过向量化", kb.ID)
			return nil
		}
		id := defaultStore.ID
		vectorStoreID = &id
	}

	for i, chunk := range chunks {
		if i >= len(vectors2D) {
			break
		}
		vec32 := make([]float32, len(vectors2D[i]))
		for j, v := range vectors2D[i] {
			vec32[j] = float32(v)
		}
		record := &entity.ChunkVector{
			UserID:          userID,
			VectorStoreID:   *vectorStoreID,
			KnowledgeBaseID: knowledge.KnowledgeBaseID,
			KnowledgeID:     knowledge.ID,
			ChunkID:         chunk.ID,
			Embedding:       pgvector.NewVector(vec32),
			ContentHash:     chunk.ContentHash,
			IsEnabled:       true,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chunk_id"}},
			UpdateAll: true,
		}).Create(record).Error; err != nil {
			return fmt.Errorf("保存向量失败: %w", err)
		}
	}
	return nil
}
