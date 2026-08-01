package service

import (
	"strings"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"

	"gorm.io/gorm"
)

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
	page, pageSize := normalizePage(request.Page, request.PageSize)
	items, total, err := s.tagRepo.ListByKnowledgeBase(kbID, request.Keyword, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	tagIDs := make([]uint, 0, len(items))
	for _, item := range items {
		tagIDs = append(tagIDs, item.ID)
	}
	knowledgeCounts := make(map[uint]int64)
	chunkCounts := make(map[uint]int64)
	if len(tagIDs) > 0 {
		type statRow struct {
			TagID uint
			Count int64
		}
		var kRows []statRow
		_ = s.db.Model(&entity.Knowledge{}).Select("tag_id, COUNT(*) as count").Where("knowledge_base_id = ? AND tag_id IN ?", kbID, tagIDs).Group("tag_id").Scan(&kRows).Error
		for _, row := range kRows {
			knowledgeCounts[row.TagID] = row.Count
		}
		var cRows []statRow
		_ = s.db.Model(&entity.Chunk{}).Select("tag_id, COUNT(*) as count").Where("knowledge_base_id = ? AND tag_id IN ?", kbID, tagIDs).Group("tag_id").Scan(&cRows).Error
		for _, row := range cRows {
			chunkCounts[row.TagID] = row.Count
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
			ChunkCount:      chunkCounts[item.ID],
			FAQCount:        faqCounts[item.ID],
			CreatedAt:       item.CreatedAt,
			UpdatedAt:       item.UpdatedAt,
		})
	}
	return resp, total, nil
}

func (s *tagService) Create(userID, kbID uint, request *req.CreateTagRequest) (*dto.TagResponse, error) {
	if _, err := s.kbRepo.FindByUserID(userID, kbID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "标签名称不能为空")
	}
	existing, err := s.tagRepo.FindByNameAndKB(name, kbID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, bizerrors.New(bizerrors.CodeResourceAlreadyExists, "标签名称已存在")
	}
	item := &entity.Tag{
		Name:            name,
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
	var chunkCount int64
	_ = s.db.Model(&entity.Chunk{}).Where("knowledge_base_id = ? AND tag_id = ?", kbID, id).Count(&chunkCount).Error
	faqCounts, _ := s.faqRepo.CountByTagIDs(kbID, []uint{id})
	if !force && (knowledgeCount > 0 || chunkCount > 0 || faqCounts[id] > 0) {
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
