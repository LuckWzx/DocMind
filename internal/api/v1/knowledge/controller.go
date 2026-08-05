package knowledge

import (
	"docmind/internal/middleware"
	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Controller 知识库控制器
type Controller struct {
	knowledgeService service.KnowledgeService
	kbService        service.KnowledgeBaseService
	faqService       service.FAQService
}

// NewController 创建知识库控制器
func NewController(knowledgeService service.KnowledgeService, kbService service.KnowledgeBaseService, faqService service.FAQService) *Controller {
	return &Controller{
		knowledgeService: knowledgeService,
		kbService:        kbService,
		faqService:       faqService,
	}
}

// UploadKnowledgeFile 上传文件并创建知识条目
func (ctrl *Controller) UploadKnowledgeFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Unauthorized(c, "用户未登录")
		return
	}
	//var userID uint = 6

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

//===================================================================

func (ctrl *Controller) ListKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
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
	page := request.Page
	pageSize := request.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"code":      0,
		"message":   "success",
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
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

// BatchGetKnowledge 批量获取知识条目状态（供前端轮询用）
func (ctrl *Controller) BatchGetKnowledge(c *gin.Context) {
	userID := middleware.GetUserID(c)
	idsRaw := c.QueryArray("ids")
	if len(idsRaw) == 0 {
		response.Success(c, []interface{}{})
		return
	}
	ids := make([]uint, 0, len(idsRaw))
	for _, raw := range idsRaw {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, uint(id))
	}
	items, err := ctrl.kbService.BatchGetKnowledge(userID, ids)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, items)
}

// GetKnowledgeSpans 获取知识处理时间线
func (ctrl *Controller) GetKnowledgeSpans(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	spans, err := ctrl.kbService.GetKnowledgeSpans(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, spans)
}

func (ctrl *Controller) PreviewKnowledgeFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	file, err := ctrl.knowledgeService.PreviewFile(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}

	escapedName := url.PathEscape(file.FileName)
	c.Header("Content-Type", file.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", escapedName))
	c.Header("Content-Length", strconv.Itoa(len(file.Content)))
	c.Data(http.StatusOK, file.ContentType, file.Content)
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
	updates := make(map[uint][]uint, len(request.Updates))
	for k, v := range request.Updates {
		kid, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			continue
		}
		tagIDs := make([]uint, 0, len(v))
		for _, s := range v {
			id, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				continue
			}
			tagIDs = append(tagIDs, uint(id))
		}
		updates[uint(kid)] = tagIDs
	}
	if err := ctrl.kbService.UpdateKnowledgeTags(userID, updates); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) ListFAQEntries(c *gin.Context) {
	userID := middleware.GetUserID(c)
	kbID, ok := parseUintParam(c, "id")
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
	kbID, ok := parseUintParam(c, "id")
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
	kbID, ok := parseUintParam(c, "id")
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
	kbID, ok := parseUintParam(c, "id")
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
	kbID, ok := parseUintParam(c, "id")
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
	kbID, ok := parseUintParam(c, "id")
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
