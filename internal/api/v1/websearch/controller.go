package websearch

import (
	"strconv"

	"docmind/internal/middleware"
	"docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 网页搜索提供方控制器
type Controller struct {
	webSearchService service.WebSearchService
}

// NewController 创建网页搜索控制器
func NewController(webSearchService service.WebSearchService) *Controller {
	return &Controller{
		webSearchService: webSearchService,
	}
}

// List 获取当前用户的网页搜索提供方列表
// @Summary 获取网页搜索提供方列表
// @Description 获取当前用户的网页搜索提供方（按用户隔离，不分页）
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers [get]
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	list, err := ctrl.webSearchService.List(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, list)
}

// Types 获取支持的引擎类型元数据（前端动态表单驱动）
// @Summary 获取引擎类型列表
// @Description 获取支持的搜索引擎类型元数据（需要哪些配置字段）
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/types [get]
func (ctrl *Controller) Types(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	response.Success(c, ctrl.webSearchService.ProviderTypes())
}

// Get 获取网页搜索提供方详情
// @Summary 获取网页搜索提供方详情
// @Description 获取当前用户指定网页搜索提供方
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id} [get]
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
	p, err := ctrl.webSearchService.GetByUser(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, p)
}

// Create 创建网页搜索提供方
// @Summary 创建网页搜索提供方
// @Description 创建当前用户的网页搜索提供方（首个自动设为默认）
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.CreateWebSearchProviderRequest true "创建请求"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers [post]
func (ctrl *Controller) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	var req request.CreateWebSearchProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p, err := ctrl.webSearchService.Create(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, p)
}

// Update 更新网页搜索提供方（api_key 走 /credentials 子资源）
// @Summary 更新网页搜索提供方
// @Description 更新当前用户的网页搜索提供方
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Param request body request.UpdateWebSearchProviderRequest true "更新请求"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id} [put]
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
	var req request.UpdateWebSearchProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	p, err := ctrl.webSearchService.Update(userID, id, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, p)
}

// Delete 删除网页搜索提供方
// @Summary 删除网页搜索提供方
// @Description 删除当前用户指定网页搜索提供方
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id} [delete]
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
	if err := ctrl.webSearchService.Delete(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// TestProvider 测试已保存的提供方
// @Summary 测试网页搜索提供方连接
// @Description 用最小查询验证已保存提供方的可达性与凭据
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id}/test [post]
func (ctrl *Controller) TestProvider(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := ctrl.webSearchService.TestProvider(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// TestRaw 测试未保存的配置（不落库）
// @Summary 测试网页搜索配置
// @Description 用提供的配置做连通性测试（不落库，创建/编辑弹窗内联测试）
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.TestWebSearchProviderRequest true "测试请求"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/test [post]
func (ctrl *Controller) TestRaw(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	var req request.TestWebSearchProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := ctrl.webSearchService.TestRaw(&req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// UpdateCredentials 更新 api_key（密钥子资源，响应不返回密钥）
// @Summary 更新网页搜索提供方凭据
// @Description 更新 api_key，不返回密钥本身
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Param request body request.UpdateWebSearchCredentialsRequest true "凭据字段"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id}/credentials [put]
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
	var req request.UpdateWebSearchCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	apiKey := ""
	if req.APIKey != nil {
		apiKey = *req.APIKey
	}
	result, err := ctrl.webSearchService.UpdateCredentials(userID, id, apiKey)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, result)
}

// DeleteCredential 清除 api_key
// @Summary 清除网页搜索提供方凭据
// @Description 清除指定凭据字段（api_key）
// @Tags 网页搜索
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "提供方ID"
// @Param field path string true "凭据字段名 api_key"
// @Success 200 {object} response.Response
// @Router /api/v1/web-search-providers/{id}/credentials/{field} [delete]
func (ctrl *Controller) DeleteCredential(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if c.Param("field") != "api_key" {
		response.BadRequest(c, "不支持的凭据字段（仅支持 api_key）")
		return
	}
	result, err := ctrl.webSearchService.DeleteCredential(userID, id)
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
