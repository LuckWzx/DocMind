package mcp

import (
	"strconv"

	"docmind/internal/middleware"
	"docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller MCP 服务控制器
type Controller struct {
	mcpService service.MCPServiceService
}

// NewController 创建 MCP 服务控制器
func NewController(mcpService service.MCPServiceService) *Controller {
	return &Controller{
		mcpService: mcpService,
	}
}

// List 获取 MCP 服务列表
// @Summary 获取 MCP 服务列表
// @Description 获取当前用户的 MCP 服务（含系统内置），不分页返回数组
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services [get]
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	svcs, err := ctrl.mcpService.List(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, svcs)
}

// Get 获取 MCP 服务详情
// @Summary 获取 MCP 服务详情
// @Description 获取当前用户指定 MCP 服务的详情
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id} [get]
func (ctrl *Controller) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	svc, err := ctrl.mcpService.GetByID(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, svc)
}

// Create 创建 MCP 服务
// @Summary 创建 MCP 服务
// @Description 注册外部 MCP Server 连接信息
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.CreateMCPServiceRequest true "创建 MCP 服务请求"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services [post]
func (ctrl *Controller) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.CreateMCPServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	svc, err := ctrl.mcpService.Create(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, svc)
}

// Update 更新 MCP 服务
// @Summary 更新 MCP 服务
// @Description 更新外部 MCP Server 连接信息，变更后自动重连
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Param request body request.UpdateMCPServiceRequest true "更新 MCP 服务请求"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id} [put]
func (ctrl *Controller) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	var req request.UpdateMCPServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	svc, err := ctrl.mcpService.Update(userID, id, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, svc)
}

// Delete 删除 MCP 服务
// @Summary 删除 MCP 服务
// @Description 删除指定 MCP 服务并断开连接
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id} [delete]
func (ctrl *Controller) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	if err := ctrl.mcpService.Delete(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// Test 测试 MCP 服务连接
// @Summary 测试 MCP 服务连接
// @Description 连接 MCP Server 并拉取工具/资源清单验证连通性
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id}/test [post]
func (ctrl *Controller) Test(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	result, err := ctrl.mcpService.Test(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// Tools 获取 MCP 服务工具列表
// @Summary 获取 MCP 服务工具列表
// @Description 获取指定 MCP Server 暴露的工具清单（优先缓存）
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id}/tools [get]
func (ctrl *Controller) Tools(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	tools, err := ctrl.mcpService.ListTools(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, tools)
}

// Resources 获取 MCP 服务资源列表
// @Summary 获取 MCP 服务资源列表
// @Description 获取指定 MCP Server 暴露的资源清单
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id}/resources [get]
func (ctrl *Controller) Resources(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	resources, err := ctrl.mcpService.ListResources(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, resources)
}

// UpdateCredentials 更新 MCP 服务凭据（密钥子资源）
// @Summary 更新 MCP 服务凭据
// @Description 更新指定 MCP 服务的密钥字段（api_key / token），不返回密钥本身，变更后自动重连
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Param request body request.UpdateMCPCredentialsRequest true "凭据字段"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id}/credentials [put]
func (ctrl *Controller) UpdateCredentials(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	var req request.UpdateMCPCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	fields := map[string]string{}
	if req.APIKey != nil {
		fields["api_key"] = *req.APIKey
	}
	if req.Token != nil {
		fields["token"] = *req.Token
	}

	result, err := ctrl.mcpService.UpdateCredentials(userID, id, fields)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteCredentialField 清除 MCP 服务凭据字段
// @Summary 清除 MCP 服务凭据字段
// @Description 清除指定 MCP 服务的密钥字段（api_key / token），变更后自动重连
// @Tags MCP 服务
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "MCP 服务ID"
// @Param field path string true "凭据字段名 api_key/token"
// @Success 200 {object} response.Response
// @Router /api/v1/mcp-services/{id}/credentials/{field} [delete]
func (ctrl *Controller) DeleteCredentialField(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseID(c)
	if !ok {
		return
	}

	result, err := ctrl.mcpService.DeleteCredentialField(userID, id, c.Param("field"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// parseID 解析路径参数 id
func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID 参数")
		return 0, false
	}
	return uint(id), true
}
