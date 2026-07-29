package knowledge

import (
	"net/http"
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
	kbID, ok := parseUintParam(c, "kbId")
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

func (ctrl *Controller) ListKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.KnowledgeListRequest
	_ = c.ShouldBindQuery(&request)
	items, total, err := ctrl.kbService.ListKnowledge(userID, kbID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    0,
		"message": "success",
		"data":    items,
		"total":   total,
	})
}

func (ctrl *Controller) GetKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := ctrl.kbService.GetKnowledge(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) ListKnowledgeChunks(c *gin.Context) {
	userID := middleware.GetUserID(c)
	knowledgeID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "25"))
	items, total, err := ctrl.kbService.ListKnowledgeChunks(userID, knowledgeID, page, pageSize)
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    0,
		"message": "success",
		"data":    items,
		"total":   total,
	})
}

func (ctrl *Controller) ReparseKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.ReparseKnowledgeRequest
	_ = c.ShouldBindJSON(&request)
	item, err := ctrl.kbService.ReparseKnowledge(userID, id, request.ProcessConfig)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) DeleteKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := ctrl.kbService.DeleteKnowledge(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) UpdateKnowledgeTags(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var request req.KnowledgeTagBatchUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.kbService.UpdateKnowledgeTags(userID, request.Updates); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) ListFAQEntries(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.FAQListRequest
	_ = c.ShouldBindQuery(&request)
	items, total, err := ctrl.faqService.List(userID, kbID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"data": items, "total": total})
}

func (ctrl *Controller) CreateFAQEntry(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.FAQEntryUpsertRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.faqService.Create(userID, kbID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) UpdateFAQEntry(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	id, ok := parseUintParam(c, "entryId")
	if !ok {
		return
	}
	var request req.FAQEntryUpsertRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := ctrl.faqService.Update(userID, kbID, id, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, item)
}

func (ctrl *Controller) BatchUpsertFAQEntries(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.FAQEntriesUpsertRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.faqService.BatchUpsert(userID, kbID, &request); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) BatchUpdateFAQFields(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.FAQEntryFieldsBatchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.faqService.BatchUpdateFields(userID, kbID, &request); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) DeleteFAQEntries(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	var request req.FAQDeleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.faqService.DeleteBatch(userID, kbID, request.IDs); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) ExportFAQEntries(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
	if !ok {
		return
	}
	raw, err := ctrl.faqService.Export(userID, kbID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=faq_entries.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", raw)
}

func (ctrl *Controller) ListTags(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
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
	response.Success(c, gin.H{"data": items, "total": total})
}

func (ctrl *Controller) CreateTag(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "kbId")
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
	kbID, ok := parseUintParam(c, "kbId")
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
	kbID, ok := parseUintParam(c, "kbId")
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
