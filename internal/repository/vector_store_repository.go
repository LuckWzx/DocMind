package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

// vectorStoreRepository 向量存储仓储实现
type vectorStoreRepository struct {
	db *gorm.DB
}

// NewVectorStoreRepository 创建向量存储仓储
func NewVectorStoreRepository(db *gorm.DB) VectorStoreRepository {
	return &vectorStoreRepository{db: db}
}

// FindByID 根据 ID 查询向量存储
func (r *vectorStoreRepository) FindByID(id uint) (*entity.VectorStore, error) {
	var store entity.VectorStore
	err := r.db.First(&store, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &store, nil
}

// FindByUserAndID 根据用户与 ID 查询向量存储
func (r *vectorStoreRepository) FindByUserAndID(userID, id uint) (*entity.VectorStore, error) {
	var store entity.VectorStore
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&store).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &store, nil
}

// Create 创建向量存储
func (r *vectorStoreRepository) Create(store *entity.VectorStore) error {
	return r.db.Create(store).Error
}

// Update 更新向量存储
func (r *vectorStoreRepository) Update(store *entity.VectorStore) error {
	return r.db.Save(store).Error
}

// Delete 删除向量存储
func (r *vectorStoreRepository) Delete(id uint) error {
	return r.db.Delete(&entity.VectorStore{}, id).Error
}

// ListByUser 分页查询用户向量存储
func (r *vectorStoreRepository) ListByUser(userID uint, offset, limit int) ([]*entity.VectorStore, int64, error) {
	var stores []*entity.VectorStore
	var total int64

	if err := r.db.Model(&entity.VectorStore{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&stores).Error
	if err != nil {
		return nil, 0, err
	}

	return stores, total, nil
}
