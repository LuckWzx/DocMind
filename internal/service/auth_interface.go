package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
)

// AuthService 认证服务接口
type AuthService interface {
	// Login 用户登录
	Login(req *request.LoginRequest) (*dto.LoginResponse, error)
	// RefreshToken 刷新 Token
	RefreshToken(token string) (string, error)
}
