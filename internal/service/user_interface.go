package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/pkg/response"

	"cloudque/internal/model/entity"
)

// UserService 用户服务接口
type UserService interface {
	// Register 用户注册
	Register(req *request.RegisterRequest) error
	// GetUserByID 根据 ID 获取用户
	GetUserByID(id uint) (*entity.User, error)
	// UpdateUser 更新用户信息
	UpdateUser(id uint, req *request.UpdateUserRequest) error
	// ChangePassword 修改密码
	ChangePassword(id uint, req *request.ChangePasswordRequest) error
	// GetUserResponse 获取用户响应
	GetUserResponse(user *entity.User) *dto.UserResponse
	// ListUsers 分页获取用户列表
	ListUsers(req *request.UserListRequest) (*response.PageResponse, error)
}
