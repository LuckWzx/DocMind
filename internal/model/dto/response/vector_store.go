package response

import "time"

// VectorStoreResponse 向量存储响应
type VectorStoreResponse struct {
	ID               uint      `json:"id"`
	UserID           uint      `json:"user_id"`
	Name             string    `json:"name"`
	EngineType       string    `json:"engine_type"`
	ConnectionConfig string    `json:"connection_config"`
	IndexConfig      string    `json:"index_config"`
	Status           int       `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
