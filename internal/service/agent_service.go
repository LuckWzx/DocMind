package service

import (
	"fmt"
	"time"

	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"
)

const (
	BuiltinQuickAnswerID    = "builtin-quick-answer"
	BuiltinSmartReasoningID = "builtin-smart-reasoning"
)

// AgentService 智能体服务接口
type AgentService interface {
	ListByUser(userID uint) ([]*entity.Agent, error)
	GetByIDStr(userID uint, idStr string) (*entity.Agent, error)
	Create(userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error)
	Update(idStr string, userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error)
	Delete(idStr string, userID uint) error
	Copy(idStr string, userID uint) (*entity.Agent, error)
	// ResolveForUser 按用户视角解析智能体：内置模板为基底，叠加该用户覆盖
	ResolveForUser(userID uint, idStr string) (*entity.Agent, error)
	// ResetOverride 恢复内置智能体默认（删除用户覆盖）
	ResetOverride(userID uint, idStr string) (*entity.Agent, error)
}

type agentService struct {
	agentRepo    repository.AgentRepository
	overrideRepo repository.AgentOverrideRepository
}

// NewAgentService 创建智能体服务
func NewAgentService(agentRepo repository.AgentRepository, overrideRepo repository.AgentOverrideRepository) AgentService {
	return &agentService{agentRepo: agentRepo, overrideRepo: overrideRepo}
}

// applyOverride 将用户覆盖合并到内置智能体模板上（返回新的合并结果）
func applyOverride(agent *entity.Agent, ov *entity.AgentOverride) *entity.Agent {
	merged := *agent
	merged.HasOverride = true
	if ov.Name != "" {
		merged.Name = ov.Name
	}
	merged.Description = ov.Description
	merged.Avatar = ov.Avatar
	merged.Config = ov.Config
	return &merged
}

func (s *agentService) ListByUser(userID uint) ([]*entity.Agent, error) {
	agents, err := s.agentRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	// 批量加载当前用户对内置智能体的覆盖
	overrides, err := s.overrideRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	ovMap := make(map[string]*entity.AgentOverride, len(overrides))
	for _, ov := range overrides {
		ovMap[ov.AgentID] = ov
	}
	for _, agent := range agents {
		if !agent.IsBuiltin {
			continue
		}
		if ov, ok := ovMap[agent.IDStr]; ok {
			merged := applyOverride(agent, ov)
			*agent = *merged
		}
	}
	return agents, nil
}

func (s *agentService) GetByIDStr(userID uint, idStr string) (*entity.Agent, error) {
	return s.ResolveForUser(userID, idStr)
}

func (s *agentService) ResolveForUser(userID uint, idStr string) (*entity.Agent, error) {
	agent, err := s.agentRepo.FindByIDStr(idStr)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询智能体失败", err)
	}
	if agent == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}
	// 仅内置智能体参与用户覆盖合并；自定义智能体直接返回
	if !agent.IsBuiltin {
		return agent, nil
	}
	// 覆盖表统一以 id_str 为键：调用方可能传数字主键（旧会话/共享入口绑定），
	// FindByIDStr 兜底按主键命中后必须用 agent.IDStr 匹配覆盖，否则用户个性化配置失效
	ov, err := s.overrideRepo.Find(userID, agent.IDStr)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询智能体覆盖失败", err)
	}
	if ov == nil {
		return agent, nil
	}
	return applyOverride(agent, ov), nil
}

func (s *agentService) Create(userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error) {
	if name == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "名称不能为空")
	}

	idStr := fmt.Sprintf("agent-%d-%d", userID, time.Now().UnixMilli())
	agent := &entity.Agent{
		UserID:      userID,
		IDStr:       idStr,
		Name:        name,
		Description: description,
		Avatar:      avatar,
		IsBuiltin:   false,
	}
	if config != nil {
		agent.Config = *config
	}

	if err := s.agentRepo.Create(agent); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建智能体失败", err)
	}
	return agent, nil
}

func (s *agentService) Update(idStr string, userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error) {
	agent, err := s.agentRepo.FindByIDStr(idStr)
	if err != nil || agent == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}
	if agent.IsBuiltin {
		// 内置智能体：个性化配置写入用户覆盖表，不修改全局模板（避免影响其他用户）
		if config == nil {
			return s.ResolveForUser(userID, idStr)
		}
		if err := s.overrideRepo.Upsert(userID, idStr, name, description, avatar, config); err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新智能体失败", err)
		}
		return s.ResolveForUser(userID, idStr)
	}
	if agent.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}

	if name != "" {
		agent.Name = name
	}
	agent.Description = description
	agent.Avatar = avatar
	if config != nil {
		agent.Config = *config
	}

	if err := s.agentRepo.Update(agent); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新智能体失败", err)
	}
	return agent, nil
}

func (s *agentService) ResetOverride(userID uint, idStr string) (*entity.Agent, error) {
	agent, err := s.agentRepo.FindByIDStr(idStr)
	if err != nil || agent == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}
	if !agent.IsBuiltin {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "仅内置智能体支持恢复默认")
	}
	if err := s.overrideRepo.Delete(userID, idStr); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "恢复默认失败", err)
	}
	return agent, nil
}

func (s *agentService) Delete(idStr string, userID uint) error {
	agent, err := s.agentRepo.FindByIDStr(idStr)
	if err != nil || agent == nil {
		return bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}
	if agent.IsBuiltin {
		return bizerrors.New(bizerrors.CodeInvalidParam, "内置智能体不可删除")
	}
	if agent.UserID != userID {
		return bizerrors.New(bizerrors.CodeForbidden, "无权操作")
	}
	return s.agentRepo.Delete(agent.ID)
}

func (s *agentService) Copy(idStr string, userID uint) (*entity.Agent, error) {
	src, err := s.ResolveForUser(userID, idStr)
	if err != nil || src == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}

	newIDStr := fmt.Sprintf("agent-%d-%d", userID, time.Now().UnixMilli())
	copy := &entity.Agent{
		UserID:      userID,
		IDStr:       newIDStr,
		Name:        src.Name + " (副本)",
		Description: src.Description,
		Avatar:      src.Avatar,
		IsBuiltin:   false,
		Config:      src.Config,
	}

	if err := s.agentRepo.Create(copy); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "复制智能体失败", err)
	}
	return copy, nil
}

// SeedBuiltinAgents 确保内置智能体存在
func SeedBuiltinAgents(agentRepo repository.AgentRepository) error {
	defaultTemp := 0.7
	defaultMaxTokens := 2048
	defaultFalse := false
	defaultTrue := true
	smartTemp := 0.8
	smartMaxTokens := 4096
	smartMaxIter := 10

	quickAnswer := &entity.Agent{
		UserID:      0,
		IDStr:       BuiltinQuickAnswerID,
		Name:        "快速问答",
		Description: "基于知识库的 RAG 问答，快速准确地回答问题",
		IsBuiltin:   true,
		Config: entity.AgentConfig{
			AgentMode:           "quick-answer",
			SystemPrompt:        "你是一个知识库问答助手，请根据检索到的文档内容准确回答用户的问题。",
			Temperature:         &defaultTemp,
			MaxCompletionTokens: &defaultMaxTokens,
			EmbeddingTopK:       5,
			VectorThreshold:     0.5,
			RerankTopK:          5,
			MultiTurnEnabled:    &defaultFalse,
			HistoryTurns:        0,
			// 系统默认：全部 MCP 服务与技能可用（空值语义等同 all，显式声明避免前端归一到 none）
			MCPSelectionMode:    "all",
			SkillsSelectionMode: "all",
		},
	}

	smartReasoning := &entity.Agent{
		UserID:      0,
		IDStr:       BuiltinSmartReasoningID,
		Name:        "智能推理",
		Description: "ReAct 推理框架，支持多步思考与工具调用",
		IsBuiltin:   true,
		Config: entity.AgentConfig{
			AgentMode:           "smart-reasoning",
			SystemPrompt:        "你是一个智能推理助手，能够进行多步思考和推理。当需要查询知识库中的信息时，请使用知识库搜索工具获取相关内容，然后基于检索结果进行分析和回答。请先思考、再检索、最后给出结论。",
			Temperature:         &smartTemp,
			MaxCompletionTokens: &smartMaxTokens,
			MaxIterations:       &smartMaxIter,
			ReflectionEnabled:   &defaultTrue,
			EmbeddingTopK:       5,
			VectorThreshold:     0.5,
			RerankTopK:          5,
			MultiTurnEnabled:    &defaultTrue,
			HistoryTurns:        3,
			// 系统默认：全部 MCP 服务与技能可用（空值语义等同 all，显式声明避免前端归一到 none）
			MCPSelectionMode:    "all",
			SkillsSelectionMode: "all",
		},
	}

	if err := agentRepo.EnsureBuiltin(quickAnswer); err != nil {
		return err
	}
	return agentRepo.EnsureBuiltin(smartReasoning)
}
