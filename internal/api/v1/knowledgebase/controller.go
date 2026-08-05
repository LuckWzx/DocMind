package knowledgebase

import (
	"strconv"

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

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	raw := c.Param(key)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		response.BadRequest(c, "非法参数: "+key)
		return 0, false
	}
	return uint(value), true
}
