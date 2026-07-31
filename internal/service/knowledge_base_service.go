package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"
	"strings"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"

	pgvector "github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type knowledgeBaseService struct {
	db              *gorm.DB
	kbRepo          repository.KnowledgeBaseRepository
	knowledgeRepo   repository.KnowledgeRepository
	faqRepo         repository.FAQRepository
	tagRepo         repository.TagRepository
	vectorStoreRepo repository.VectorStoreRepository
	gateway         KnowledgePipelineGateway
}

func NewKnowledgeBaseService(
	db *gorm.DB,
	kbRepo repository.KnowledgeBaseRepository,
	knowledgeRepo repository.KnowledgeRepository,
	faqRepo repository.FAQRepository,
	tagRepo repository.TagRepository,
	vectorStoreRepo repository.VectorStoreRepository,
	gateway KnowledgePipelineGateway,
) KnowledgeBaseService {
	return &knowledgeBaseService{
		db:              db,
		kbRepo:          kbRepo,
		knowledgeRepo:   knowledgeRepo,
		faqRepo:         faqRepo,
		tagRepo:         tagRepo,
		vectorStoreRepo: vectorStoreRepo,
		gateway:         gateway,
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
	if err := decodeInto(request.Config.ChunkingConfig, &kb.ChunkingConfig); err != nil {
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

func (s *knowledgeBaseService) UploadFile(userID, kbID uint, fileHeader *multipart.FileHeader, processConfig string, tagID *uint) (*dto.KnowledgeResponse, error) {
	kb, err := s.kbRepo.FindByUserID(userID, kbID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	sourceRef, err := s.gateway.StageUpload(context.Background(), fileHeader)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "暂存上传文件失败", err)
	}

	item := &entity.Knowledge{
		Title:           strings.TrimSuffix(fileHeader.Filename, filepathExt(fileHeader.Filename)),
		FileName:        fileHeader.Filename,
		Type:            entity.KnowledgeTypeFile,
		Source:          "web",
		ParseStatus:     entity.KnowledgeParseStatusPending,
		SummaryStatus:   "completed",
		ProcessingStage: "queued",
		KnowledgeBaseID: kbID,
		FileURL:         sourceRef,
		FileType:        strings.TrimPrefix(strings.ToLower(filepathExt(fileHeader.Filename)), "."),
		FileSize:        fileHeader.Size,
		ErrorMessage:    "",
	}
	if tagID != nil {
		item.TagID = *tagID
	}
	if strings.TrimSpace(processConfig) != "" {
		item.ProcessConfig = entity.JSON([]byte(processConfig))
	}
	if err := s.knowledgeRepo.Create(item); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建知识记录失败", err)
	}

	go s.processKnowledge(item.ID)
	return s.toKnowledgeResponse(item, nil), nil
}

func (s *knowledgeBaseService) ListKnowledge(userID, kbID uint, request *req.KnowledgeListRequest) ([]*dto.KnowledgeResponse, int64, error) {
	_, err := s.Get(userID, kbID)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(request.Page, request.PageSize)
	items, total, err := s.knowledgeRepo.List(repository.KnowledgeListFilter{
		KnowledgeBaseID: kbID,
		TagIDs:          parseUintCSV(request.TagIDs),
		Keyword:         request.Keyword,
		FileType:        request.FileType,
		ParseStatus:     request.ParseStatus,
		Source:          request.Source,
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

func (s *knowledgeBaseService) GetKnowledge(userID, id uint) (*dto.KnowledgeResponse, error) {
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
	return s.toKnowledgeResponse(item, tag), nil
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

func (s *knowledgeBaseService) ReparseKnowledge(userID, id uint, processConfig string) (*dto.KnowledgeResponse, error) {
	item, err := s.knowledgeRepo.FindByID(id)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识详情失败", err)
	}
	if item == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	if _, err := s.Get(userID, item.KnowledgeBaseID); err != nil {
		return nil, err
	}
	item.ParseStatus = entity.KnowledgeParseStatusPending
	item.ProcessingStage = "queued"
	item.ErrorMessage = ""
	if strings.TrimSpace(processConfig) != "" {
		item.ProcessConfig = entity.JSON([]byte(processConfig))
	}
	if err := s.knowledgeRepo.Update(item); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新知识状态失败", err)
	}
	go s.processKnowledge(item.ID)
	tag, _ := s.loadTag(item.TagID)
	return s.toKnowledgeResponse(item, tag), nil
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

func (s *knowledgeBaseService) processKnowledge(id uint) {
	item, err := s.knowledgeRepo.FindByID(id)
	if err != nil || item == nil {
		return
	}
	item.ParseStatus = entity.KnowledgeParseStatusProcessing
	item.ProcessingStage = "parsing"
	_ = s.knowledgeRepo.Update(item)

	parsed, err := s.gateway.ParseDocument(context.Background(), item.FileURL, string(item.ProcessConfig))
	if err != nil {
		s.markKnowledgeFailed(item, err)
		return
	}

	if err := s.db.Where("knowledge_id = ?", item.ID).Delete(&entity.ChunkVector{}).Error; err != nil {
		s.markKnowledgeFailed(item, err)
		return
	}
	if err := s.db.Where("knowledge_id = ?", item.ID).Delete(&entity.Chunk{}).Error; err != nil {
		s.markKnowledgeFailed(item, err)
		return
	}

	item.ParseStatus = entity.KnowledgeParseStatusFinalizing
	item.ProcessingStage = "indexing"
	if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(parsed.Title) != "" {
		item.Title = parsed.Title
	}
	_ = s.knowledgeRepo.Update(item)

	chunkIDs := make([]uint, 0, len(parsed.Chunks))
	vectorTexts := make([]string, 0, len(parsed.Chunks))
	for idx, text := range parsed.Chunks {
		contentHash := sha256.Sum256([]byte(text))
		chunk := &entity.Chunk{
			Content:         text,
			ChunkIndex:      idx + 1,
			KnowledgeID:     item.ID,
			KnowledgeBaseID: item.KnowledgeBaseID,
			ChunkType:       entity.ChunkTypeMarkdown,
			ChunkStatus:     1,
			TagID:           item.TagID,
			ContentHash:     hex.EncodeToString(contentHash[:]),
			IsEnabled:       true,
		}
		if err := s.db.Create(chunk).Error; err != nil {
			s.markKnowledgeFailed(item, err)
			return
		}
		chunkIDs = append(chunkIDs, chunk.ID)
		vectorTexts = append(vectorTexts, text)
	}

	kb, err := s.kbRepo.FindByID(item.KnowledgeBaseID)
	if err != nil || kb == nil {
		s.markKnowledgeFailed(item, fmt.Errorf("知识库不存在"))
		return
	}
	if kb.IndexingStrategy.VectorEnabled && kb.VectorStoreID != nil {
		vectors, err := s.gateway.BuildEmbeddings(context.Background(), kb.EmbeddingModelID, vectorTexts)
		if err != nil {
			s.markKnowledgeFailed(item, err)
			return
		}
		for idx, chunkID := range chunkIDs {
			record := &entity.ChunkVector{
				UserID:          0,
				VectorStoreID:   *kb.VectorStoreID,
				KnowledgeBaseID: item.KnowledgeBaseID,
				KnowledgeID:     item.ID,
				ChunkID:         chunkID,
				Embedding:       pgvector.NewVector(vectors[idx]),
				ContentHash:     "",
				IsEnabled:       true,
			}
			if err := s.db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "chunk_id"}},
				UpdateAll: true,
			}).Create(record).Error; err != nil {
				s.markKnowledgeFailed(item, err)
				return
			}
		}
	}

	item.ParseStatus = entity.KnowledgeParseStatusCompleted
	item.ProcessingStage = "done"
	item.ErrorMessage = ""
	_ = s.knowledgeRepo.Update(item)
}

func (s *knowledgeBaseService) markKnowledgeFailed(item *entity.Knowledge, err error) {
	item.ParseStatus = entity.KnowledgeParseStatusFailed
	item.ProcessingStage = "failed"
	item.ErrorMessage = err.Error()
	_ = s.knowledgeRepo.Update(item)
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

func (s *knowledgeBaseService) countKnowledge(kbID uint) (int64, error) {
	var count int64
	err := s.db.Model(&entity.Knowledge{}).Where("knowledge_base_id = ?", kbID).Count(&count).Error
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

func filepathExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx == -1 {
		return ""
	}
	return name[idx:]
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

type tagService struct {
	db      *gorm.DB
	kbRepo  repository.KnowledgeBaseRepository
	tagRepo repository.TagRepository
	faqRepo repository.FAQRepository
}

func NewTagService(db *gorm.DB, kbRepo repository.KnowledgeBaseRepository, tagRepo repository.TagRepository, faqRepo repository.FAQRepository) TagService {
	return &tagService{db: db, kbRepo: kbRepo, tagRepo: tagRepo, faqRepo: faqRepo}
}

func (s *tagService) List(userID, kbID uint, request *req.TagListRequest) ([]*dto.TagResponse, int64, error) {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return nil, 0, err
	}
	items, err := s.tagRepo.ListByKnowledgeBase(kbID, request.Keyword)
	if err != nil {
		return nil, 0, err
	}
	tagIDs := make([]uint, 0, len(items))
	for _, item := range items {
		tagIDs = append(tagIDs, item.ID)
	}
	knowledgeCounts := make(map[uint]int64)
	if len(tagIDs) > 0 {
		type statRow struct {
			TagID uint
			Count int64
		}
		var rows []statRow
		_ = s.db.Model(&entity.Knowledge{}).Select("tag_id, COUNT(*) as count").Where("knowledge_base_id = ? AND tag_id IN ?", kbID, tagIDs).Group("tag_id").Scan(&rows).Error
		for _, row := range rows {
			knowledgeCounts[row.TagID] = row.Count
		}
	}
	faqCounts, _ := s.faqRepo.CountByTagIDs(kbID, tagIDs)
	resp := make([]*dto.TagResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, &dto.TagResponse{
			ID:              item.ID,
			SeqID:           item.ID,
			Name:            item.Name,
			Color:           item.Color,
			KnowledgeBaseID: item.KnowledgeBaseID,
			SortOrder:       item.SortOrder,
			KnowledgeCount:  knowledgeCounts[item.ID],
			ChunkCount:      faqCounts[item.ID],
			FAQCount:        faqCounts[item.ID],
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	return resp, int64(len(resp)), nil
}

func (s *tagService) Create(userID, kbID uint, request *req.CreateTagRequest) (*dto.TagResponse, error) {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return nil, err
	}
	item := &entity.Tag{
		Name:            strings.TrimSpace(request.Name),
		Color:           strings.TrimSpace(request.Color),
		KnowledgeBaseID: kbID,
		SortOrder:       request.SortOrder,
	}
	if err := s.tagRepo.Create(item); err != nil {
		return nil, err
	}
	return &dto.TagResponse{
		ID:              item.ID,
		SeqID:           item.ID,
		Name:            item.Name,
		Color:           item.Color,
		KnowledgeBaseID: item.KnowledgeBaseID,
		SortOrder:       item.SortOrder,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}, nil
}

func (s *tagService) Update(userID, kbID, id uint, request *req.UpdateTagRequest) (*dto.TagResponse, error) {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return nil, err
	}
	item, err := s.tagRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil || item.KnowledgeBaseID != kbID {
		return nil, bizerrors.ErrResourceNotFound
	}
	if request.Name != "" {
		item.Name = strings.TrimSpace(request.Name)
	}
	if request.Color != "" {
		item.Color = strings.TrimSpace(request.Color)
	}
	if request.SortOrder != nil {
		item.SortOrder = *request.SortOrder
	}
	if err := s.tagRepo.Update(item); err != nil {
		return nil, err
	}
	return &dto.TagResponse{
		ID:              item.ID,
		SeqID:           item.ID,
		Name:            item.Name,
		Color:           item.Color,
		KnowledgeBaseID: item.KnowledgeBaseID,
		SortOrder:       item.SortOrder,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}, nil
}

func (s *tagService) Delete(userID, kbID, id uint, force bool) error {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return err
	}
	item, err := s.tagRepo.FindByID(id)
	if err != nil {
		return err
	}
	if item == nil || item.KnowledgeBaseID != kbID {
		return bizerrors.ErrResourceNotFound
	}
	var knowledgeCount int64
	_ = s.db.Model(&entity.Knowledge{}).Where("knowledge_base_id = ? AND tag_id = ?", kbID, id).Count(&knowledgeCount).Error
	faqCounts, _ := s.faqRepo.CountByTagIDs(kbID, []uint{id})
	if !force && (knowledgeCount > 0 || faqCounts[id] > 0) {
		return bizerrors.New(bizerrors.CodeInvalidParam, "标签已被引用，请使用 force 删除")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.Knowledge{}).Where("knowledge_base_id = ? AND tag_id = ?", kbID, id).Update("tag_id", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.Chunk{}).Where("knowledge_base_id = ? AND tag_id = ?", kbID, id).Update("tag_id", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&entity.FAQ{}).Where("knowledge_base_id = ? AND tag_id = ?", kbID, id).Update("tag_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Tag{}, id).Error
	})
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
