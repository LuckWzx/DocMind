package knowledgebase

import (
	"strconv"
	"strings"

	"docmind/internal/middleware"
	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

type Controller struct {
	kbService  service.KnowledgeBaseService
	faqService service.FAQService
	tagService service.TagService
}

func NewController(kbService service.KnowledgeBaseService, faqService service.FAQService, tagService service.TagService) *Controller {
	return &Controller{
		kbService:  kbService,
		faqService: faqService,
		tagService: tagService,
	}
}

func (ctrl *Controller) ListKnowledgeBases(c *gin.Context) {
	userID := middleware.GetUserID(c)
	items, err := ctrl.kbService.List(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, items)
}

func (ctrl *Controller) CreateKnowledgeBase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var request req.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.kbService.Create(userID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) GetKnowledgeBase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := ctrl.kbService.Get(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) UpdateKnowledgeBase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.kbService.Update(userID, id, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) DeleteKnowledgeBase(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := ctrl.kbService.Delete(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) UploadKnowledgeFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少上传文件")
		return
	}
	processConfig := c.PostForm("process_config")
	tagID := parseTagID(c.PostForm("tag_id"), c.PostForm("tag_ids"))
	item, err := ctrl.kbService.UploadFile(userID, kbID, fileHeader, processConfig, tagID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	raw := c.Param(key)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		response.BadRequest(c, "非法参数: "+key)
		return 0, false
	}
	return uint(value), true
}

func parseTagID(values ...string) *uint {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, ",") {
			raw = strings.TrimSpace(strings.Split(raw, ",")[0])
		}
		value, err := strconv.ParseUint(raw, 10, 64)
		if err == nil {
			parsed := uint(value)
			return &parsed
		}
	}
	return nil
}
