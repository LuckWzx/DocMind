package models

import (
	"docmind/internal/middleware"
	"encoding/json"
	"strconv"

	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 模型管理控制器
type Controller struct {
	modelService service.ModelService
}

// NewController 创建模型管理控制器
func NewController(modelService service.ModelService) *Controller {
	return &Controller{modelService: modelService}
}

func (ctrl *Controller) ListModels(c *gin.Context) {
	modelType := c.Query("type")
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.ListModels(userID, modelType)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) CreateModel(c *gin.Context) {
	var request req.UpsertModelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.CreateModel(userID, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) GetModel(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.GetModel(userID, id)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) UpdateModel(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.UpsertModelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.UpdateModel(userID, id, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) DeleteModel(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	if err := ctrl.modelService.DeleteModel(userID, id); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctrl *Controller) ListProviders(c *gin.Context) {
	data := ctrl.modelService.ListProviders(c.Query("model_type"))
	response.Success(c, data)
}

// ListMissingContextWindows 上下文大小缺失模型清单（供定期补足内置映射表）
func (ctrl *Controller) ListMissingContextWindows(c *gin.Context) {
	data, err := ctrl.modelService.ListMissingContextWindows()
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) PutModelCredentials(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var request req.PutModelCredentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.PutModelCredentials(userID, id, &request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) DeleteModelCredentialField(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.DeleteModelCredentialField(userID, id, c.Param("field"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) DebugModel(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	input := c.PostForm("input")
	documentsRaw := c.PostForm("documents")
	optionsRaw := c.PostForm("options")

	var documents []string
	if documentsRaw != "" {
		if err := json.Unmarshal([]byte(documentsRaw), &documents); err != nil {
			response.BadRequest(c, "documents 格式错误")
			return
		}
	}

	options := map[string]interface{}{}
	if optionsRaw != "" {
		if err := json.Unmarshal([]byte(optionsRaw), &options); err != nil {
			response.BadRequest(c, "options 格式错误")
			return
		}
	}

	fileHeader, _ := c.FormFile("file")
	userID := middleware.GetUserID(c)
	data, err := ctrl.modelService.DebugModel(userID, id, input, documents, options, fileHeader)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) GetDocMindCloudStatus(c *gin.Context) {
	data, err := ctrl.modelService.GetDocMindCloudStatus()
	if err != nil {
		response.BizError(c, err)
		return
	}
	c.JSON(200, data)
}

func (ctrl *Controller) SaveDocMindCloudCredentials(c *gin.Context) {
	var request req.SaveDocMindCloudCredentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := ctrl.modelService.SaveDocMindCloudCredentials(request.AppID, request.AppSecret); err != nil {
		response.BizError(c, err)
		return
	}
	response.SuccessWithMessage(c, "凭据保存成功", nil)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(value), true
}
