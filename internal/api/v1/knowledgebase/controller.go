package knowledgebase

import (
	"strconv"

	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 知识库控制器
type Controller struct {
	knowledgeService service.KnowledgeService
}

// NewController 创建知识库控制器
func NewController(knowledgeService service.KnowledgeService) *Controller {
	return &Controller{knowledgeService: knowledgeService}
}

// UploadKnowledgeFile 上传文件并创建知识条目
func (ctrl *Controller) UploadKnowledgeFile(c *gin.Context) {
	//userID := middleware.GetUserID(c)
	//if userID == 0 {
	//	response.Unauthorized(c, "用户未登录")
	//	return
	//}
	var userID uint = 6

	knowledgeBaseID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请上传文件")
		return
	}

	data, err := ctrl.knowledgeService.UploadFile(userID, knowledgeBaseID, fileHeader)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(value), true
}
