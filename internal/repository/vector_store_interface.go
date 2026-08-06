package repository

import "docmind/internal/model/entity"

// VectorStoreRepository 向量存储仓储接口
type VectorStoreRepository interface {
	FindByID(id uint) (*entity.VectorStore, error)
	FindByUserAndID(userID, id uint) (*entity.VectorStore, error)
	Create(store *entity.VectorStore) error
	Update(store *entity.VectorStore) error
	Delete(id uint) error
	ListByUser(userID uint, offset, limit int) ([]*entity.VectorStore, int64, error)
	FirstOrCreateGlobalDefault() (*entity.VectorStore, error)
	FindGlobalDefault() (*entity.VectorStore, error)
}
