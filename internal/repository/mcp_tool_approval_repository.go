package repository

import (
	"docmind/internal/model/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mcpToolApprovalRepository MCP 工具审批设置仓储实现
type mcpToolApprovalRepository struct {
	db *gorm.DB
}

// NewMCPToolApprovalRepository 创建 MCP 工具审批设置仓储
func NewMCPToolApprovalRepository(db *gorm.DB) MCPToolApprovalRepository {
	return &mcpToolApprovalRepository{db: db}
}

// ListByUserAndService 查询用户对指定服务的全部审批设置
func (r *mcpToolApprovalRepository) ListByUserAndService(userID, serviceID uint) ([]*entity.MCPToolApproval, error) {
	var rows []*entity.MCPToolApproval
	err := r.db.Where("user_id = ? AND service_id = ?", userID, serviceID).
		Order("tool_name ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListByUser 查询用户全部审批设置（Agent 工具集构建时一次拉全量）
func (r *mcpToolApprovalRepository) ListByUser(userID uint) ([]*entity.MCPToolApproval, error) {
	var rows []*entity.MCPToolApproval
	err := r.db.Where("user_id = ?", userID).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Upsert 设置（或清除）某工具的审批要求，按 (UserID, ServiceID, ToolName) 唯一
// requireApproval=false 时保留一行 false（语义明确），也可由上层直接删除
func (r *mcpToolApprovalRepository) Upsert(userID, serviceID uint, toolName string, requireApproval bool) error {
	row := &entity.MCPToolApproval{
		UserID:          userID,
		ServiceID:       serviceID,
		ToolName:        toolName,
		RequireApproval: requireApproval,
	}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "service_id"}, {Name: "tool_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"require_approval", "updated_at"}),
	}).Create(row).Error
}
