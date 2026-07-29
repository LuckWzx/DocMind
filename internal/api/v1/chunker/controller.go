package chunker

import (
	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 分块预览控制器
type Controller struct {
	chunkerService service.ChunkerService
}

// NewController 创建分块控制器
func NewController(chunkerService service.ChunkerService) *Controller {
	return &Controller{chunkerService: chunkerService}
}

// PreviewChunking 预览 Markdown 分块效果
func (ctrl *Controller) PreviewChunking(c *gin.Context) {
	var request req.PreviewChunkingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	data, err := ctrl.chunkerService.Preview(&request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}
