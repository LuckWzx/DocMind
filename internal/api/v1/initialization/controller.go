package initialization

import (
	req "docmind/internal/model/dto/request"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 初始化与模型探测控制器
type Controller struct {
	modelService service.ModelService
}

// NewController 创建控制器
func NewController(modelService service.ModelService) *Controller {
	return &Controller{modelService: modelService}
}

func (ctrl *Controller) CheckRemoteModel(c *gin.Context) {
	var request req.ModelTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.CheckRemoteModel(&request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) TestEmbeddingModel(c *gin.Context) {
	var request req.ModelTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.TestEmbeddingModel(&request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) CheckRerankModel(c *gin.Context) {
	var request req.ModelTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.CheckRerankModel(&request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) CheckASRModel(c *gin.Context) {
	var request req.ModelTestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.CheckASRModel(&request)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) GetOllamaStatus(c *gin.Context) {
	data, err := ctrl.modelService.GetOllamaStatus()
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) ListOllamaModels(c *gin.Context) {
	data, err := ctrl.modelService.ListOllamaModels()
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"models": data})
}

func (ctrl *Controller) CheckOllamaModels(c *gin.Context) {
	var request struct {
		Models []string `json:"models"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.CheckOllamaModels(request.Models)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{"models": data})
}

func (ctrl *Controller) DownloadOllamaModel(c *gin.Context) {
	var request struct {
		ModelName string `json:"modelName"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	data, err := ctrl.modelService.DownloadOllamaModel(request.ModelName)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, gin.H{
		"taskId":    data.ID,
		"modelName": data.ModelName,
		"status":    data.Status,
		"progress":  data.Progress,
	})
}

func (ctrl *Controller) GetOllamaDownloadProgress(c *gin.Context) {
	data, err := ctrl.modelService.GetOllamaDownloadProgress(c.Param("taskId"))
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}

func (ctrl *Controller) ListOllamaDownloadTasks(c *gin.Context) {
	data, err := ctrl.modelService.ListOllamaDownloadTasks()
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, data)
}
