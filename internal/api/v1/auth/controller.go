package auth

import (
	"cloudque/internal/middleware"
	"cloudque/internal/model/dto/request"
	"cloudque/internal/service"
	"cloudque/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 认证控制器
type Controller struct {
	authService service.AuthService
	userService service.UserService
}

// NewController 创建认证控制器
func NewController(authService service.AuthService, userService service.UserService) *Controller {
	return &Controller{
		authService: authService,
		userService: userService,
	}
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建新用户账号
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.RegisterRequest true "注册信息"
// @Success 200 {object} response.Response
// @Router /api/v1/auth/register [post]
func (ctrl *Controller) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := ctrl.userService.Register(&req); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "注册成功", nil)
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取 Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.LoginRequest true "登录信息"
// @Success 200 {object} response.Response{data=response.LoginResponse}
// @Router /api/v1/auth/login [post]
func (ctrl *Controller) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.Login(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 获取当前登录用户的详细信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=response.UserResponse}
// @Router /api/v1/auth/me [get]
func (ctrl *Controller) GetCurrentUser(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "未登录")
		return
	}

	resp, err := ctrl.authService.GetUserFromToken(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// RefreshToken 刷新 Token
// @Summary 刷新 Token
// @Description 使用 Refresh Token 获取新的 Access Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body request.RefreshTokenRequest true "Refresh Token"
// @Success 200 {object} response.Response{data=response.RefreshTokenResponse}
// @Router /api/v1/auth/refresh [post]
func (ctrl *Controller) RefreshToken(c *gin.Context) {
	var req request.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := ctrl.authService.RefreshToken(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}

	response.Success(c, resp)
}

// Logout 用户登出
// @Summary 用户登出
// @Description 退出当前登录状态
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /api/v1/auth/logout [post]
func (ctrl *Controller) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "未登录")
		return
	}

	if err := ctrl.authService.Logout(userID); err != nil {
		response.BizError(c, err)
		return
	}

	response.SuccessWithMessage(c, "登出成功", nil)
}
