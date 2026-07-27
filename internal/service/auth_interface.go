package service

import (
	"docmind/internal/model/dto/request"
	"docmind/internal/model/dto/response"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录
	Login(req *request.LoginRequest) (*response.LoginResponse, error)
	// RefreshToken 刷新 Token
	RefreshToken(req *request.RefreshTokenRequest) (*response.RefreshTokenResponse, error)
	// Logout 用户登出
	Logout(userID uint) error
	// GetUserFromToken 从 Token 获取用户信息
	GetUserFromToken(userID uint) (*response.UserResponse, error)
}
