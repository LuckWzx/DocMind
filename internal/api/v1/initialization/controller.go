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

// modelTestFn 模型探测函数签名，统一的入参/出参类型
type modelTestFn func(request *req.ModelTestRequest) (map[string]interface{}, error)

// handleModelTest 统一处理模型探测请求：绑定 JSON → 调探测函数 → 返回结果
func handleModelTest(fn modelTestFn) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request req.ModelTestRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		data, err := fn(&request)
		if err != nil {
			response.BizError(c, err)
			return
		}
		response.Success(c, data)
	}
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
