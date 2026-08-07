package repository

import "docmind/internal/model/entity"

// ModelContextWindowMissingRepository 上下文大小缺失记录仓储（待补足映射表的模型清单）
type ModelContextWindowMissingRepository interface {
	// Upsert 写入或更新缺失记录（每个模型至多一行，按 model_id 冲突更新）
	Upsert(record *entity.ModelContextWindowMissing) error
	// ClearByModelID 清除指定模型的缺失记录（上下文已确定或模型已删除，硬删除）
	ClearByModelID(modelID uint) error
	// ListAll 返回全部缺失记录（按创建时间倒序）
	ListAll() ([]*entity.ModelContextWindowMissing, error)
}
