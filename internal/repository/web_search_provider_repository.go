package repository

import (
	"errors"

	"docmind/internal/model/entity"

	"gorm.io/gorm"
)

// webSearchProviderRepository 网页搜索提供方仓储实现
type webSearchProviderRepository struct {
	db *gorm.DB
}

// NewWebSearchProviderRepository 创建网页搜索提供方仓储
func NewWebSearchProviderRepository(db *gorm.DB) WebSearchProviderRepository {
	return &webSearchProviderRepository{db: db}
}

// FindByUserAndID 根据用户与 ID 查询（严格归属校验，跨用户不可见）
func (r *webSearchProviderRepository) FindByUserAndID(userID, id uint) (*entity.WebSearchProvider, error) {
	var p entity.WebSearchProvider
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// Create 创建提供方
func (r *webSearchProviderRepository) Create(p *entity.WebSearchProvider) error {
	return r.db.Create(p).Error
}

// Update 更新提供方
func (r *webSearchProviderRepository) Update(p *entity.WebSearchProvider) error {
	return r.db.Save(p).Error
}

// Delete 删除提供方
func (r *webSearchProviderRepository) Delete(id uint) error {
	return r.db.Delete(&entity.WebSearchProvider{}, id).Error
}

// ListByUser 查询用户全部提供方，按创建时间倒序
func (r *webSearchProviderRepository) ListByUser(userID uint) ([]*entity.WebSearchProvider, error) {
	var list []*entity.WebSearchProvider
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ListEnabledByUser 查询用户已启用提供方，按创建时间倒序
func (r *webSearchProviderRepository) ListEnabledByUser(userID uint) ([]*entity.WebSearchProvider, error) {
	var list []*entity.WebSearchProvider
	err := r.db.Where("user_id = ? AND is_enabled = true", userID).
		Order("created_at DESC").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// ClearDefault 清除用户全部默认标记（设置新默认前调用）
func (r *webSearchProviderRepository) ClearDefault(userID uint) error {
	return r.db.Model(&entity.WebSearchProvider{}).
		Where("user_id = ? AND is_default = true", userID).
		Update("is_default", false).Error
}
