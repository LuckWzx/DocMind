package service

import (
	"context"

	dto "docmind/internal/model/dto/response"
)

// SystemService 系统信息服务：版本/构建信息/引擎与运行状态（只读）
type SystemService interface {
	// Info 获取系统信息（GET /api/v1/system/info）
	Info(ctx context.Context) *dto.SystemInfoResponse
}
