package system

import (
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 系统信息控制器
type Controller struct {
	systemService service.SystemService
}

// NewController 创建系统信息控制器
func NewController(systemService service.SystemService) *Controller {
	return &Controller{
		systemService: systemService,
	}
}

// Info 获取系统信息
// @Summary 获取系统信息
// @Description 获取系统版本、构建信息、检索/存储引擎与运行状态（设置页系统信息视图）
// @Tags 系统信息
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} response.Response
// @Router /api/v1/system/info [get]
func (ctrl *Controller) Info(c *gin.Context) {
	info := ctrl.systemService.Info(c.Request.Context())
	response.Success(c, info)
}
