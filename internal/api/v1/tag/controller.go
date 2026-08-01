package tag

import (
	"strconv"

	"docmind/internal/middleware"
	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 标签控制器
type Controller struct {
	tagService service.TagService
}

// NewController 创建标签控制器
func NewController(tagService service.TagService) *Controller {
	return &Controller{tagService: tagService}
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(value), true
}

func (ctrl *Controller) ListTags(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.TagListRequest
	_ = c.ShouldBindQuery(&request)
	items, total, err := ctrl.tagService.List(userID, kbID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	page := request.Page
	pageSize := request.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	response.Success(c, gin.H{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (ctrl *Controller) CreateTag(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.CreateTagRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.tagService.Create(userID, kbID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) UpdateTag(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	id, ok := parseUintParam(c, "tagId")
	if !ok {
		return
	}
	var request req.UpdateTagRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.tagService.Update(userID, kbID, id, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) DeleteTag(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	id, ok := parseUintParam(c, "tagId")
	if !ok {
		return
	}
	force := c.Query("force") == "true"
	if err := ctrl.tagService.Delete(userID, kbID, id, force); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}
