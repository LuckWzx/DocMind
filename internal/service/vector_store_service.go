package service

import (
	"encoding/json"

	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"
	pkgresponse "docmind/pkg/response"
)

// vectorStoreService 向量存储服务实现
type vectorStoreService struct {
	vectorStoreRepo repository.VectorStoreRepository
}

// NewVectorStoreService 创建向量存储服务
func NewVectorStoreService(vectorStoreRepo repository.VectorStoreRepository) VectorStoreService {
	return &vectorStoreService{
		vectorStoreRepo: vectorStoreRepo,
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
	_, err := s.GetEntityByID(userID, id)
	if err != nil {
		return err
	}
	// 阶段一先完成配置与绑定链路，具体驱动连接测试后续接入。
	return nil
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

func jsonString(raw entity.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	return string(raw)
}
