package service

import (
	"time"

	"cloudque/internal/model/dto/request"
	"cloudque/internal/model/dto/response"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/jwt"

	"golang.org/x/crypto/bcrypt"
)

const refreshTokenTTL = 7 * 24 * time.Hour // refresh_token 有效期 7 天

// authService 认证服务实现
type authService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	userService      UserService
}

// NewAuthService 创建认证服务
func NewAuthService(
	userRepo repository.UserRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	userService UserService,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		userService:      userService,
	}
}

// Login 用户登录
func (s *authService) Login(req *request.LoginRequest) (*response.LoginResponse, error) {
	// 根据邮箱查找用户
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 检查用户状态
	if user.Status != 1 {
		return nil, bizerrors.ErrUserDisabled
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, bizerrors.ErrInvalidCredentials
	}

	// 生成 Access Token
	token, err := jwt.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "生成 Token 失败", err)
	}

	// 生成 Refresh Token
	refreshToken, err := jwt.GenerateRefreshToken(user.ID, user.Username)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "生成 Refresh Token 失败", err)
	}

	// 保存 Refresh Token 到 Redis（TTL 自动过期）
	if err := s.refreshTokenRepo.Save(refreshToken, user.ID, refreshTokenTTL); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存 Refresh Token 失败", err)
	}

	// 构建响应
	userResp := s.userService.GetUserResponse(user)
	return &response.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         *userResp,
	}, nil
}

// RefreshToken 刷新 Token
func (s *authService) RefreshToken(req *request.RefreshTokenRequest) (*response.RefreshTokenResponse, error) {
	// 解析 Refresh Token，验证签名和过期时间
	claims, err := jwt.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, bizerrors.ErrInvalidToken
	}

	// 从 Redis 中查找 token 是否存在（防止重复使用）
	userID, err := s.refreshTokenRepo.FindByToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}
	if userID == 0 {
		// token 不存在或已过期
		return nil, bizerrors.ErrInvalidToken
	}

	// 生成新的 Access Token
	token, err := jwt.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "生成 Token 失败", err)
	}

	// 生成新的 Refresh Token
	newRefreshToken, err := jwt.GenerateRefreshToken(claims.UserID, claims.Username)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "生成 Refresh Token 失败", err)
	}

	// 删除旧的 Refresh Token（Rotation 策略：每次刷新都换新 token）
	if err := s.refreshTokenRepo.Delete(req.RefreshToken); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "删除旧 Token 失败", err)
	}

	// 保存新的 Refresh Token
	if err := s.refreshTokenRepo.Save(newRefreshToken, claims.UserID, refreshTokenTTL); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存新 Token 失败", err)
	}

	return &response.RefreshTokenResponse{
		Token:        token,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout 用户登出
func (s *authService) Logout(userID uint) error {
	// 删除用户的所有 Refresh Token
	if err := s.refreshTokenRepo.DeleteByUserID(userID); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "登出失败", err)
	}
	return nil
}

// GetUserFromToken 从 Token 获取用户信息
func (s *authService) GetUserFromToken(userID uint) (*response.UserResponse, error) {
	user, err := s.userService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return s.userService.GetUserResponse(user), nil
}
