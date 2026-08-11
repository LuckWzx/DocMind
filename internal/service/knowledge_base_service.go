package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"

	"gorm.io/gorm"
)

type knowledgeBaseService struct {
	db              *gorm.DB
	kbRepo          repository.KnowledgeBaseRepository
	knowledgeRepo   repository.KnowledgeRepository
	faqRepo         repository.FAQRepository
	tagRepo         repository.TagRepository
	vectorStoreRepo repository.VectorStoreRepository
}

func NewKnowledgeBaseService(
	db *gorm.DB,
	kbRepo repository.KnowledgeBaseRepository,
	knowledgeRepo repository.KnowledgeRepository,
	faqRepo repository.FAQRepository,
	tagRepo repository.TagRepository,
	vectorStoreRepo repository.VectorStoreRepository,
) KnowledgeBaseService {
	return &knowledgeBaseService{
		db:              db,
		kbRepo:          kbRepo,
		knowledgeRepo:   knowledgeRepo,
		faqRepo:         faqRepo,
		tagRepo:         tagRepo,
		vectorStoreRepo: vectorStoreRepo,
	}
}

func (s *knowledgeBaseService) List(userID uint) ([]*dto.KnowledgeBaseResponse, error) {
	items, err := s.kbRepo.ListByUser(userID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	resp := make([]*dto.KnowledgeBaseResponse, 0, len(items))
	for _, item := range items {
		count, _ := s.countKnowledge(item.ID)
		resp = append(resp, s.toKnowledgeBaseResponse(item, count))
	}
	return resp, nil
}

func (s *knowledgeBaseService) Create(userID uint, request *req.CreateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error) {
	kb := &entity.KnowledgeBase{
		UserID:           userID,
		Name:             strings.TrimSpace(request.Name),
		Description:      strings.TrimSpace(request.Description),
		Type:             normalizeKBType(request.Type),
		EmbeddingModelID: strings.TrimSpace(request.EmbeddingModelID),
		SummaryModelID:   strings.TrimSpace(request.SummaryModelID),
		VectorStoreID:    request.VectorStoreID,
	}
	if err := decodeInto(request.ChunkingConfig, &kb.ChunkingConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "chunking_config 格式错误", err)
	}
	if err := decodeInto(request.ExtractConfig, &kb.ExtractConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "extract_config 格式错误", err)
	}
	if err := decodeInto(request.FAQConfig, &kb.FAQConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "faq_config 格式错误", err)
	}
	if err := decodeInto(request.IndexingStrategy, &kb.IndexingStrategy); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "indexing_strategy 格式错误", err)
	}
	if err := decodeInto(request.VLMConfig, &kb.VLMConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "vlm_config 格式错误", err)
	}
	if err := decodeInto(request.ASRConfig, &kb.ASRConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "asr_config 格式错误", err)
	}
	// 多模态开关同步到分块配置，解析链路统一读 EnableMM
	if kb.VLMConfig != nil {
		kb.ChunkingConfig.EnableMM = kb.VLMConfig.Enabled
	}
	if kb.StorageProviderConfig == nil {
		kb.StorageProviderConfig = &entity.StorageProviderConfig{Provider: "mock"}
	}
	if err := s.validateVectorStore(userID, kb.VectorStoreID); err != nil {
		return nil, err
	}
	if err := s.kbRepo.Create(kb); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建知识库失败", err)
	}
	return s.toKnowledgeBaseResponse(kb, 0), nil
}

func (s *knowledgeBaseService) Get(userID, id uint) (*dto.KnowledgeBaseResponse, error) {
	kb, err := s.kbRepo.FindByUserID(userID, id)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	count, _ := s.countKnowledge(kb.ID)
	return s.toKnowledgeBaseResponse(kb, count), nil
}

func (s *knowledgeBaseService) Update(userID, id uint, request *req.UpdateKnowledgeBaseRequest) (*dto.KnowledgeBaseResponse, error) {
	kb, err := s.kbRepo.FindByUserID(userID, id)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	kb.Name = strings.TrimSpace(request.Name)
	kb.Description = strings.TrimSpace(request.Description)
	// 模型配置（与创建接口的顶层字段对齐）
	kb.EmbeddingModelID = strings.TrimSpace(request.EmbeddingModelID)
	kb.SummaryModelID = strings.TrimSpace(request.SummaryModelID)
	// 分块配置：顶层 chunking_config 优先，兼容旧的 config.chunking_config 嵌套传参
	chunking := request.ChunkingConfig
	if chunking == nil {
		chunking = request.Config.ChunkingConfig
	}
	if err := decodeInto(chunking, &kb.ChunkingConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "chunking_config 格式错误", err)
	}
	if err := decodeInto(request.Config.ExtractConfig, &kb.ExtractConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "extract_config 格式错误", err)
	}
	if err := decodeInto(request.Config.FAQConfig, &kb.FAQConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "faq_config 格式错误", err)
	}
	if err := decodeInto(request.Config.IndexingStrategy, &kb.IndexingStrategy); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "indexing_strategy 格式错误", err)
	}
	// 存储提供方：只合并显式传入的字段，避免覆盖既有 bucket/密钥等细节
	if request.StorageProviderConfig != nil {
		if kb.StorageProviderConfig == nil {
			kb.StorageProviderConfig = &entity.StorageProviderConfig{}
		}
		mergeStorageProvider(kb.StorageProviderConfig, request.StorageProviderConfig)
	}
	// 多模态/语音识别配置：不传时保留原值；开关同步到分块配置
	if err := decodeInto(request.VLMConfig, &kb.VLMConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "vlm_config 格式错误", err)
	}
	if err := decodeInto(request.ASRConfig, &kb.ASRConfig); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInvalidParam, "asr_config 格式错误", err)
	}
	if kb.VLMConfig != nil {
		kb.ChunkingConfig.EnableMM = kb.VLMConfig.Enabled
	}
	if err := s.kbRepo.Update(kb); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新知识库失败", err)
	}
	count, _ := s.countKnowledge(kb.ID)
	return s.toKnowledgeBaseResponse(kb, count), nil
}

func (s *knowledgeBaseService) Delete(userID, id uint) error {
	kb, err := s.kbRepo.FindByUserID(userID, id)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return bizerrors.ErrResourceNotFound
	}
	// 硬删除策略：chunks 物理删除会同步从 Tantivy BM25 索引移除，
	// 故知识库下所有关联数据（向量记录/分块/知识条目/FAQ/标签）统一级联物理删除
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&entity.ChunkVector{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&entity.Chunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&entity.Knowledge{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&entity.FAQ{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_base_id = ?", id).Delete(&entity.Tag{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.KnowledgeBase{}, id).Error
	})
}

func (s *knowledgeBaseService) ListKnowledge(userID, kbID uint, request *req.KnowledgeListRequest) ([]*dto.KnowledgeResponse, int64, error) {
	_, err := s.Get(userID, kbID)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(request.Page, request.PageSize)
	items, total, err := s.knowledgeRepo.List(repository.KnowledgeListFilter{
		KnowledgeBaseID: kbID,
		TagIDs:          parseUintCSV(request.TagID),
		Keyword:         request.Keyword,
		FileType:        request.FileType,
		ParseStatus:     request.ParseStatus,
		Source:          request.Source,
		StartTime:       parseFlexibleTime(request.StartTime),
		EndTime:         parseFlexibleTime(request.EndTime),
		Offset:          (page - 1) * pageSize,
		Limit:           pageSize,
	})
	if err != nil {
		return nil, 0, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识列表失败", err)
	}
	resp := make([]*dto.KnowledgeResponse, 0, len(items))
	for _, item := range items {
		tag, _ := s.loadTag(item.TagID)
		resp = append(resp, s.toKnowledgeResponse(item, tag))
	}
	return resp, total, nil
}

func (s *knowledgeBaseService) GetKnowledge(userID, id uint) (*dto.KnowledgeDetailResponse, error) {
	item, err := s.knowledgeRepo.FindByID(id)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识详情失败", err)
	}
	if item == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	kb, err := s.kbRepo.FindByUserID(userID, item.KnowledgeBaseID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	tag, _ := s.loadTag(item.TagID)
	chunkCount, err := s.countKnowledgeChunks(item.ID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "统计知识分块失败", err)
	}
	return s.toKnowledgeDetailResponse(item, tag, chunkCount), nil
}

// BatchGetKnowledge 批量获取知识条目状态（供前端轮询用）
func (s *knowledgeBaseService) BatchGetKnowledge(userID uint, ids []uint) ([]*dto.KnowledgeResponse, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	items, err := s.knowledgeRepo.FindByIDs(ids)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "批量查询知识失败", err)
	}
	resp := make([]*dto.KnowledgeResponse, 0, len(items))
	for _, item := range items {
		tag, _ := s.loadTag(item.TagID)
		resp = append(resp, s.toKnowledgeResponse(item, tag))
	}
	return resp, nil
}

// GetKnowledgeSpans 获取知识条目的处理时间线（供前端轮询用）
func (s *knowledgeBaseService) GetKnowledgeSpans(userID, id uint) (*dto.KnowledgeSpansResponse, error) {
	item, err := s.knowledgeRepo.FindByID(id)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识失败", err)
	}
	if item == nil {
		return nil, bizerrors.ErrResourceNotFound
	}

	// 映射 parse_status → trace status
	traceStatus := "ok"
	switch item.ParseStatus {
	case parseStatusPending:
		traceStatus = "pending"
	case parseStatusProcessing:
		traceStatus = "running"
	case parseStatusFailed:
		traceStatus = "error"
	}

	trace := &dto.SpanNode{
		Name:   "knowledge-processing",
		Kind:   "process",
		Status: traceStatus,
	}

	var lastError *dto.SpanNode
	if item.ParseStatus == parseStatusFailed && item.ErrorMessage != "" {
		lastError = &dto.SpanNode{
			Name:         "parse-error",
			ErrorCode:    "PARSE_FAILED",
			ErrorMessage: item.ErrorMessage,
		}
	}

	return &dto.KnowledgeSpansResponse{
		KnowledgeID:   item.ID,
		Attempt:       1,
		LatestAttempt: 1,
		ParseStatus:   item.ParseStatus,
		Trace:         trace,
		LastError:     lastError,
	}, nil
}

func (s *knowledgeBaseService) ListKnowledgeChunks(userID, knowledgeID uint, page, pageSize int) ([]map[string]interface{}, int64, error) {
	item, err := s.knowledgeRepo.FindByID(knowledgeID)
	if err != nil {
		return nil, 0, err
	}
	if item == nil {
		return nil, 0, bizerrors.ErrResourceNotFound
	}
	if _, err := s.Get(userID, item.KnowledgeBaseID); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	var total int64
	if err := s.db.Model(&entity.Chunk{}).Where("knowledge_id = ?", knowledgeID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var chunks []*entity.Chunk
	err = s.db.Where("knowledge_id = ?", knowledgeID).Order("chunk_index ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&chunks).Error
	if err != nil {
		return nil, 0, err
	}
	resp := make([]map[string]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		resp = append(resp, map[string]interface{}{
			"id":          chunk.ID,
			"content":     chunk.Content,
			"chunk_index": chunk.ChunkIndex,
			"chunk_type":  chunk.ChunkType,
		})
	}
	return resp, total, nil
}

func (s *knowledgeBaseService) UpdateKnowledgeTags(userID uint, updates map[uint][]uint) error {
	for knowledgeID, tagIDs := range updates {
		item, err := s.knowledgeRepo.FindByID(knowledgeID)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		if _, err := s.Get(userID, item.KnowledgeBaseID); err != nil {
			return err
		}
		var tagID uint
		if len(tagIDs) > 0 {
			tagID = tagIDs[0]
		}
		item.TagID = tagID
		if err := s.knowledgeRepo.Update(item); err != nil {
			return err
		}
		if err := s.db.Model(&entity.Chunk{}).Where("knowledge_id = ?", item.ID).Update("tag_id", tagID).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *knowledgeBaseService) DeleteKnowledge(userID, id uint) error {
	item, err := s.knowledgeRepo.FindByID(id)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识详情失败", err)
	}
	if item == nil {
		return bizerrors.ErrResourceNotFound
	}
	if _, err := s.Get(userID, item.KnowledgeBaseID); err != nil {
		return err
	}
	// 硬删除策略：与知识库删除一致，chunks 物理删除避免 BM25 索引死文档堆积
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("knowledge_id = ?", id).Delete(&entity.ChunkVector{}).Error; err != nil {
			return err
		}
		if err := tx.Where("knowledge_id = ?", id).Delete(&entity.Chunk{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Knowledge{}, id).Error
	})
}

func (s *knowledgeBaseService) validateVectorStore(userID uint, id *uint) error {
	if id == nil {
		return nil
	}
	store, err := s.vectorStoreRepo.FindByUserAndID(userID, *id)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询向量存储失败", err)
	}
	if store == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "向量存储不存在")
	}
	return nil
}

func (s *knowledgeBaseService) toKnowledgeBaseResponse(item *entity.KnowledgeBase, count int64) *dto.KnowledgeBaseResponse {
	storageCfg := item.StorageProviderConfig
	if storageCfg == nil {
		storageCfg = &entity.StorageProviderConfig{Provider: "mock"}
	}
	return &dto.KnowledgeBaseResponse{
		ID:                    item.ID,
		Name:                  item.Name,
		Description:           item.Description,
		Type:                  item.Type,
		EmbeddingModelID:      item.EmbeddingModelID,
		SummaryModelID:        item.SummaryModelID,
		VectorStoreID:         item.VectorStoreID,
		ChunkingConfig:        item.ChunkingConfig,
		ExtractConfig:         item.ExtractConfig,
		FAQConfig:             item.FAQConfig,
		StorageProviderConfig: storageCfg,
		IndexingStrategy:      item.IndexingStrategy,
		VLMConfig:             item.VLMConfig,
		ASRConfig:             item.ASRConfig,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
		DocumentCount:         count,
	}
}

func (s *knowledgeBaseService) toKnowledgeResponse(item *entity.Knowledge, tag *entity.Tag) *dto.KnowledgeResponse {
	tags := make([]dto.KnowledgeTagLite, 0, 1)
	if tag != nil {
		tags = append(tags, dto.KnowledgeTagLite{
			ID:    tag.ID,
			Name:  tag.Name,
			Color: tag.Color,
		})
	}
	return &dto.KnowledgeResponse{
		ID:              item.ID,
		KnowledgeBaseID: item.KnowledgeBaseID,
		Title:           item.Title,
		FileName:        item.FileName,
		Description:     item.Description,
		Type:            item.Type,
		Source:          item.Source,
		Channel:         item.Channel,
		ParseStatus:     item.ParseStatus,
		SummaryStatus:   item.SummaryStatus,
		FileType:        item.FileType,
		FileSize:        item.FileSize,
		TagID:           item.TagID,
		Tags:            tags,
		ErrorMessage:    item.ErrorMessage,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (s *knowledgeBaseService) toKnowledgeDetailResponse(item *entity.Knowledge, tag *entity.Tag, chunkCount int64) *dto.KnowledgeDetailResponse {
	tags := make([]dto.KnowledgeTagLite, 0, 1)
	if tag != nil {
		tags = append(tags, dto.KnowledgeTagLite{
			ID:    tag.ID,
			Name:  tag.Name,
			Color: tag.Color,
		})
	}
	return &dto.KnowledgeDetailResponse{
		ID:              item.ID,
		KnowledgeBaseID: item.KnowledgeBaseID,
		Title:           item.Title,
		FileName:        item.FileName,
		Description:     item.Description,
		Type:            item.Type,
		Source:          item.Source,
		Channel:         item.Channel,
		ParseStatus:     item.ParseStatus,
		SummaryStatus:   item.SummaryStatus,
		ProcessingStage: item.ProcessingStage,
		FileType:        item.FileType,
		FileSize:        item.FileSize,
		FileURL:         item.FileURL,
		FilePath:        item.FileURL,
		TagID:           item.TagID,
		Tags:            tags,
		ProcessConfig:   item.ProcessConfig,
		ErrorMessage:    item.ErrorMessage,
		ChunkCount:      chunkCount,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (s *knowledgeBaseService) countKnowledge(kbID uint) (int64, error) {
	var count int64
	err := s.db.Model(&entity.Knowledge{}).Where("knowledge_base_id = ?", kbID).Count(&count).Error
	return count, err
}

func (s *knowledgeBaseService) countKnowledgeChunks(knowledgeID uint) (int64, error) {
	var count int64
	err := s.db.Model(&entity.Chunk{}).Where("knowledge_id = ?", knowledgeID).Count(&count).Error
	return count, err
}

func (s *knowledgeBaseService) loadTag(tagID uint) (*entity.Tag, error) {
	if tagID == 0 {
		return nil, nil
	}
	return s.tagRepo.FindByID(tagID)
}

func normalizeKBType(value string) string {
	if value == "faq" {
		return "faq"
	}
	return "document"
}

// mergeStorageProvider 将 src 中非空字段合并进 dst，用于更新知识库时
// 只覆盖显式传入的存储配置项，避免误清空既有 bucket/密钥等细节。
func mergeStorageProvider(dst, src *entity.StorageProviderConfig) {
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.BucketName != "" {
		dst.BucketName = src.BucketName
	}
	if src.Endpoint != "" {
		dst.Endpoint = src.Endpoint
	}
	if src.AccessKey != "" {
		dst.AccessKey = src.AccessKey
	}
	if src.SecretKey != "" {
		dst.SecretKey = src.SecretKey
	}
	if src.Region != "" {
		dst.Region = src.Region
	}
	if src.Extra != nil {
		dst.Extra = src.Extra
	}
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func decodeInto(input interface{}, out interface{}) error {
	if input == nil {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if string(raw) == "null" || string(raw) == "{}" {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func parseUintCSV(raw string) []uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]uint, 0, len(parts))
	for _, item := range parts {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			result = append(result, uint(parsed))
		}
	}
	return result
}

func parseFlexibleTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

type faqService struct {
	db      *gorm.DB
	kbRepo  repository.KnowledgeBaseRepository
	faqRepo repository.FAQRepository
}

func NewFAQService(db *gorm.DB, kbRepo repository.KnowledgeBaseRepository, faqRepo repository.FAQRepository) FAQService {
	return &faqService{db: db, kbRepo: kbRepo, faqRepo: faqRepo}
}

func (s *faqService) List(userID, kbID uint, request *req.FAQListRequest) ([]*dto.FAQResponse, int64, error) {
	kb, err := s.kbRepo.FindByUserID(userID, kbID)
	if err != nil {
		return nil, 0, err
	}
	if kb == nil {
		return nil, 0, bizerrors.ErrResourceNotFound
	}
	page, pageSize := normalizePage(request.Page, request.PageSize)
	items, total, err := s.faqRepo.List(repository.FAQListFilter{
		KnowledgeBaseID: kbID,
		TagID:           request.TagID,
		Keyword:         request.Keyword,
		Offset:          (page - 1) * pageSize,
		Limit:           pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	resp := make([]*dto.FAQResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, faqToResponse(item))
	}
	return resp, total, nil
}

func (s *faqService) Create(userID, kbID uint, request *req.FAQEntryUpsertRequest) (*dto.FAQResponse, error) {
	kb, err := s.kbRepo.FindByUserID(userID, kbID)
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	item := &entity.FAQ{
		UserID:            userID,
		KnowledgeBaseID:   kbID,
		StandardQuestion:  strings.TrimSpace(request.StandardQuestion),
		Answer:            strings.TrimSpace(request.Answer),
		SimilarQuestions:  entity.FAQStringList(request.SimilarQuestions),
		NegativeQuestions: entity.FAQStringList(request.NegativeQuestions),
		TagID:             request.TagID,
		IsEnabled:         request.IsEnabled == nil || *request.IsEnabled,
		IsRecommended:     request.IsRecommended != nil && *request.IsRecommended,
	}
	if err := s.faqRepo.Create(item); err != nil {
		return nil, err
	}
	return faqToResponse(item), nil
}

func (s *faqService) Update(userID, kbID, id uint, request *req.FAQEntryUpsertRequest) (*dto.FAQResponse, error) {
	item, err := s.faqRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil || item.KnowledgeBaseID != kbID {
		return nil, bizerrors.ErrResourceNotFound
	}
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return nil, err
	}
	item.StandardQuestion = strings.TrimSpace(request.StandardQuestion)
	item.Answer = strings.TrimSpace(request.Answer)
	item.SimilarQuestions = entity.FAQStringList(request.SimilarQuestions)
	item.NegativeQuestions = entity.FAQStringList(request.NegativeQuestions)
	item.TagID = request.TagID
	if request.IsEnabled != nil {
		item.IsEnabled = *request.IsEnabled
	}
	if request.IsRecommended != nil {
		item.IsRecommended = *request.IsRecommended
	}
	if err := s.faqRepo.Update(item); err != nil {
		return nil, err
	}
	return faqToResponse(item), nil
}

func (s *faqService) BatchUpsert(userID, kbID uint, request *req.FAQEntriesUpsertRequest) error {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return err
	}
	if request.Mode == "replace" {
		if err := s.faqRepo.DeleteByKnowledgeBase(kbID); err != nil {
			return err
		}
	}
	for _, entry := range request.Entries {
		if _, err := s.Create(userID, kbID, &entry); err != nil {
			return err
		}
	}
	return nil
}

func (s *faqService) BatchUpdateFields(userID, kbID uint, request *req.FAQEntryFieldsBatchRequest) error {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return err
	}
	for id, fields := range request.ByID {
		item, err := s.faqRepo.FindByID(id)
		if err != nil {
			return err
		}
		if item == nil || item.KnowledgeBaseID != kbID {
			continue
		}
		if fields.IsEnabled != nil {
			item.IsEnabled = *fields.IsEnabled
		}
		if fields.IsRecommended != nil {
			item.IsRecommended = *fields.IsRecommended
		}
		if fields.TagID != nil {
			item.TagID = fields.TagID
		}
		if err := s.faqRepo.Update(item); err != nil {
			return err
		}
	}
	return nil
}

func (s *faqService) DeleteBatch(userID, kbID uint, ids []uint) error {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return err
	}
	return s.faqRepo.DeleteBatch(ids)
}

func (s *faqService) Export(userID, kbID uint) ([]byte, error) {
	items, _, err := s.List(userID, kbID, &req.FAQListRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	_ = writer.Write([]string{"问题", "答案", "相似问题", "反例问题", "tag_id", "is_enabled", "is_recommended"})
	for _, item := range items {
		_ = writer.Write([]string{
			item.StandardQuestion,
			item.Answer,
			strings.Join(item.SimilarQuestions, "##"),
			strings.Join(item.NegativeQuestions, "##"),
			fmt.Sprintf("%v", item.TagID),
			fmt.Sprintf("%t", item.IsEnabled),
			fmt.Sprintf("%t", item.IsRecommended),
		})
	}
	writer.Flush()
	return []byte(builder.String()), writer.Error()
}

func faqToResponse(item *entity.FAQ) *dto.FAQResponse {
	return &dto.FAQResponse{
		ID:                item.ID,
		KnowledgeBaseID:   item.KnowledgeBaseID,
		StandardQuestion:  item.StandardQuestion,
		Answer:            item.Answer,
		SimilarQuestions:  []string(item.SimilarQuestions),
		NegativeQuestions: []string(item.NegativeQuestions),
		TagID:             item.TagID,
		IsEnabled:         item.IsEnabled,
		IsRecommended:     item.IsRecommended,
		SortOrder:         item.SortOrder,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}
