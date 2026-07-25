package service

import (
	"cloudque/internal/model/dto/request"
	dto "cloudque/internal/model/dto/response"
	"cloudque/internal/model/entity"
	"cloudque/internal/repository"
	bizerrors "cloudque/pkg/errors"
	"cloudque/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"net"
	"strings"
	"unicode"
)

// userService 用户服务实现
type userService struct {
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{
		userRepo: userRepo,
	}
}

// validatePassword 校验密码复杂度：至少8位，包含大小写字母和数字
func validatePassword(password string) error {
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}
	if !hasUpper {
		return bizerrors.New(bizerrors.CodeInvalidParam, "密码必须包含大写字母")
	}
	if !hasLower {
		return bizerrors.New(bizerrors.CodeInvalidParam, "密码必须包含小写字母")
	}
	if !hasDigit {
		return bizerrors.New(bizerrors.CodeInvalidParam, "密码必须包含数字")
	}
	return nil
}

// validateEmailDomain 校验邮箱域名是否有 MX 记录（能收邮件）
func validateEmailDomain(email string) error {
	// 提取域名（@ 后面的部分）
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return bizerrors.New(bizerrors.CodeInvalidParam, "邮箱格式不正确")
	}
	domain := strings.ToLower(parts[1])

	// 查询 MX 记录
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return bizerrors.New(bizerrors.CodeInvalidParam, "邮箱域名不存在，请检查邮箱地址")
	}
	return nil
}

// Register 用户注册
func (s *userService) Register(req *request.RegisterRequest) error {
	// 校验密码复杂度
	if err := validatePassword(req.Password); err != nil {
		return err
	}

	// 校验邮箱域名是否存在
	if err := validateEmailDomain(req.Email); err != nil {
		return err
	}

	// 检查用户名是否存在
	exists, err := s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.ErrUserAlreadyExists
	}

	// 检查邮箱是否存在
	exists, err = s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return err
	}
	if exists {
		return bizerrors.New(bizerrors.CodeUserAlreadyExists, "邮箱已被注册")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 创建用户
	user := &entity.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Email:    req.Email,
		Nickname: req.Username, // 默认使用用户名作为昵称
		Status:   1,            // 正常状态
	}

	if err := s.userRepo.Create(user); err != nil {
		return err
	}

	return nil
}

// GetUserByID 根据 ID 获取用户
func (s *userService) GetUserByID(id uint) (*entity.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, bizerrors.ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (s *userService) UpdateUser(id uint, req *request.UpdateUserRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新字段
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	return s.userRepo.Update(user)
}

// ChangePassword 修改密码
func (s *userService) ChangePassword(id uint, req *request.ChangePasswordRequest) error {
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return bizerrors.ErrInvalidCredentials
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.Password = string(hashedPassword)
	return s.userRepo.Update(user)
}

// GetUserResponse 获取用户响应
func (s *userService) GetUserResponse(user *entity.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// ListUsers 分页获取用户列表
func (s *userService) ListUsers(req *request.UserListRequest) (*response.PageResponse, error) {
	// 参数标准化
	page := req.Page
	if page < 1 {
		page = 1
	}
	size := req.Size
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	// 计算偏移量
	offset := (page - 1) * size

	// 查询数据
	users, total, err := s.userRepo.List(offset, size)
	if err != nil {
		return nil, err
	}

	// 转换为响应 DTO
	list := make([]*dto.UserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, s.GetUserResponse(user))
	}

	return response.NewPageResponse(list, total, page, size), nil
}

// CheckUsernameExists 检查用户名是否存在
func (s *userService) CheckUsernameExists(username string) (bool, error) {
	return s.userRepo.ExistsByUsername(username)
}

// CheckEmailExists 检查邮箱是否存在
func (s *userService) CheckEmailExists(email string) (bool, error) {
	return s.userRepo.ExistsByEmail(email)
}
