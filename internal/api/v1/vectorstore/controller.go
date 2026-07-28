package vectorstore

import (
	"strconv"

	"docmind/internal/middleware"
	"docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 向量存储控制器
type Controller struct {
	vectorStoreService service.VectorStoreService
}

// NewController 创建向量存储控制器
func NewController(vectorStoreService service.VectorStoreService) *Controller {
	return &Controller{
		vectorStoreService: vectorStoreService,
	}
}

// List 获取向量存储列表
// @Summary 获取向量存储列表
// @Description 分页获取当前用户的向量存储实例
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int true "页码" minimum(1)
// @Param size query int true "每页大小" minimum(1) maximum(100)
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores [get]
func (ctrl *Controller) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.VectorStoreListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	pageResp, err := ctrl.vectorStoreService.List(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, pageResp)
}

// Create 创建向量存储
// @Summary 创建向量存储
// @Description 创建当前用户的向量存储实例
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body request.CreateVectorStoreRequest true "创建向量存储请求"
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores [post]
func (ctrl *Controller) Create(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req request.CreateVectorStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	store, err := ctrl.vectorStoreService.Create(userID, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, store)
}

// Get 获取向量存储详情
// @Summary 获取向量存储详情
// @Description 获取当前用户的向量存储实例详情
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "向量存储ID"
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores/{id} [get]
func (ctrl *Controller) Get(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	store, err := ctrl.vectorStoreService.GetByID(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, store)
}

// Update 更新向量存储
// @Summary 更新向量存储
// @Description 更新当前用户的向量存储实例
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "向量存储ID"
// @Param request body request.UpdateVectorStoreRequest true "更新向量存储请求"
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores/{id} [put]
func (ctrl *Controller) Update(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var req request.UpdateVectorStoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	store, err := ctrl.vectorStoreService.Update(userID, id, &req)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, store)
}

// Delete 删除向量存储
// @Summary 删除向量存储
// @Description 删除当前用户的向量存储实例
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "向量存储ID"
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores/{id} [delete]
func (ctrl *Controller) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := ctrl.vectorStoreService.Delete(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// TestConnection 测试连接
// @Summary 测试向量存储连接
// @Description 测试当前用户指定向量存储的连接可用性
// @Tags 向量存储
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "向量存储ID"
// @Success 200 {object} response.Response
// @Router /api/v1/vector-stores/{id}/test-connection [post]
func (ctrl *Controller) TestConnection(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}

	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := ctrl.vectorStoreService.TestConnection(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "连接测试通过", nil)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID 参数")
		return 0, false
	}
	return uint(value), true
}
