package service

import (
	"fmt"
	"time"

	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"
)

const (
	BuiltinQuickAnswerID = "builtin-quick-answer"
)

// AgentService 智能体服务接口
type AgentService interface {
	ListByUser(userID uint) ([]*entity.Agent, error)
	GetByIDStr(idStr string) (*entity.Agent, error)
	Create(userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error)
	Update(idStr string, userID uint, name, description, avatar string, config *entity.AgentConfig) (*entity.Agent, error)
	Delete(idStr string, userID uint) error
	Copy(idStr string, userID uint) (*entity.Agent, error)
}

type agentService struct {
	agentRepo repository.AgentRepository
}

// NewAgentService 创建智能体服务
func NewAgentService(agentRepo repository.AgentRepository) AgentService {
	return &agentService{agentRepo: agentRepo}
}

func (s *agentService) ListByUser(userID uint) ([]*entity.Agent, error) {
	return s.agentRepo.ListByUser(userID)
}

func (s *agentService) GetByIDStr(idStr string) (*entity.Agent, error) {
	agent, err := s.agentRepo.FindByIDStr(idStr)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询智能体失败", err)
	}
	if agent == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "智能体不存在")
	}
	return agent, nil
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
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "内置智能体不可编辑")
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
	src, err := s.agentRepo.FindByIDStr(idStr)
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
			MultiTurnEnabled:    false,
			HistoryTurns:        0,
		},
	}

	return agentRepo.EnsureBuiltin(quickAnswer)
}
