package agent

import (
	"docmind/internal/middleware"
	"docmind/internal/model/entity"
	"docmind/internal/service"
	"docmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// Controller 智能体控制器
type Controller struct {
	agentService service.AgentService
}

// NewController 创建智能体控制器
func NewController(agentService service.AgentService) *Controller {
	return &Controller{agentService: agentService}
}

// ListAgents 获取智能体列表
func (ctrl *Controller) ListAgents(c *gin.Context) {
	userID := middleware.GetUserID(c)
	agents, err := ctrl.agentService.ListByUser(userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	if agents == nil {
		agents = make([]*entity.Agent, 0)
	}
	response.Success(c, agents)
}

// GetAgent 获取单个智能体
func (ctrl *Controller) GetAgent(c *gin.Context) {
	idStr := c.Param("id")
	agent, err := ctrl.agentService.GetByIDStr(idStr)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, agent)
}

// CreateAgent 创建智能体
func (ctrl *Controller) CreateAgent(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Avatar      string              `json:"avatar"`
		Config      *entity.AgentConfig `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	agent, err := ctrl.agentService.Create(userID, req.Name, req.Description, req.Avatar, req.Config)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, agent)
}

// UpdateAgent 更新智能体
func (ctrl *Controller) UpdateAgent(c *gin.Context) {
	userID := middleware.GetUserID(c)
	idStr := c.Param("id")
	var req struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Avatar      string              `json:"avatar"`
		Config      *entity.AgentConfig `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	agent, err := ctrl.agentService.Update(idStr, userID, req.Name, req.Description, req.Avatar, req.Config)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, agent)
}

// DeleteAgent 删除智能体
func (ctrl *Controller) DeleteAgent(c *gin.Context) {
	userID := middleware.GetUserID(c)
	idStr := c.Param("id")
	if err := ctrl.agentService.Delete(idStr, userID); err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, nil)
}

// CopyAgent 复制智能体
func (ctrl *Controller) CopyAgent(c *gin.Context) {
	userID := middleware.GetUserID(c)
	idStr := c.Param("id")
	agent, err := ctrl.agentService.Copy(idStr, userID)
	if err != nil {
		response.BizError(c, err)
		return
	}
	response.Success(c, agent)
}
