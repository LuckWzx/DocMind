package service

import (
	"context"
	"encoding/json"
	"strings"

	"docmind/internal/mcp"
	"docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	bizerrors "docmind/pkg/errors"
)

// mcpService MCP 服务业务实现
type mcpService struct {
	repo         repository.MCPServiceRepository
	approvalRepo repository.MCPToolApprovalRepository
	manager      *mcp.Manager
}

// NewMCPService 创建 MCP 服务业务
func NewMCPService(repo repository.MCPServiceRepository, approvalRepo repository.MCPToolApprovalRepository, manager *mcp.Manager) MCPServiceService {
	return &mcpService{
		repo:         repo,
		approvalRepo: approvalRepo,
		manager:      manager,
	}
}

// List 获取 MCP 服务列表（系统内置 + 当前用户自建，不分页）
func (s *mcpService) List(userID uint) ([]*dto.MCPServiceResponse, error) {
	svcs, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	list := make([]*dto.MCPServiceResponse, 0, len(svcs))
	for _, svc := range svcs {
		list = append(list, toMCPServiceResponse(svc))
	}
	return list, nil
}

// GetByID 获取 MCP 服务详情
func (s *mcpService) GetByID(userID, id uint) (*dto.MCPServiceResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}
	return toMCPServiceResponse(svc), nil
}

// Create 创建 MCP 服务
func (s *mcpService) Create(userID uint, req *request.CreateMCPServiceRequest) (*dto.MCPServiceResponse, error) {
	if err := validateTransport(req.TransportType, req.URL, req.StdioConfig); err != nil {
		return nil, err
	}
	if err := validateAuthType(req.AuthConfig); err != nil {
		return nil, err
	}

	svc := &entity.MCPService{
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		TransportType:  req.TransportType,
		URL:            req.URL,
		Headers:        marshalJSONOrNil(req.Headers),
		AuthConfig:     marshalAuthConfig(req.AuthConfig),
		AdvancedConfig: marshalJSONOrNil(req.AdvancedConfig),
		StdioConfig:    marshalJSONOrNil(req.StdioConfig),
		EnvVars:        marshalJSONOrNil(req.EnvVars),
		Enabled:        true,
	}
	if req.Enabled != nil {
		svc.Enabled = *req.Enabled
	}

	if err := s.repo.Create(svc); err != nil {
		return nil, err
	}
	return toMCPServiceResponse(svc), nil
}

// Update 更新 MCP 服务（变更连接相关配置后强制断开，下次访问自动重连）
// 系统级（user_id=0）服务只读：配置文件是唯一事实源，不允许用户修改
func (s *mcpService) Update(userID, id uint, req *request.UpdateMCPServiceRequest) (*dto.MCPServiceResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}
	if svc.UserID == 0 {
		return nil, bizerrors.New(bizerrors.CodeForbidden, "系统级 MCP 服务只读，不可修改（由配置文件 mcp_preset_services 管理）")
	}

	if req.Name != "" {
		svc.Name = req.Name
	}
	if req.Description != "" {
		svc.Description = req.Description
	}
	if req.Enabled != nil {
		svc.Enabled = *req.Enabled
	}
	if req.TransportType != "" {
		svc.TransportType = req.TransportType
	}
	if req.URL != "" {
		svc.URL = req.URL
	}
	if req.Headers != nil {
		svc.Headers = marshalJSONOrNil(req.Headers)
	}
	if req.AuthConfig != nil {
		if err := validateAuthType(req.AuthConfig); err != nil {
			return nil, err
		}
		svc.AuthConfig = marshalAuthConfig(req.AuthConfig)
	}
	if req.AdvancedConfig != nil {
		svc.AdvancedConfig = marshalJSONOrNil(req.AdvancedConfig)
	}
	if req.StdioConfig != nil {
		svc.StdioConfig = marshalJSONOrNil(req.StdioConfig)
	}
	if req.EnvVars != nil {
		svc.EnvVars = marshalJSONOrNil(req.EnvVars)
	}

	if err := validateTransport(svc.TransportType, svc.URL, req.StdioConfig); err != nil {
		return nil, err
	}
	if err := s.repo.Update(svc); err != nil {
		return nil, err
	}
	// 连接参数可能已变更，断开缓存连接强制重建
	s.manager.Close(svc.ID)
	return toMCPServiceResponse(svc), nil
}

// Delete 删除 MCP 服务
// 系统级（user_id=0）服务只读：删除请修改配置文件 mcp_preset_services 后重启
func (s *mcpService) Delete(userID, id uint) error {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return err
	}
	if svc.UserID == 0 {
		return bizerrors.New(bizerrors.CodeForbidden, "系统级 MCP 服务只读，不可删除（由配置文件 mcp_preset_services 管理）")
	}
	s.manager.Close(svc.ID)
	return s.repo.Delete(svc.ID)
}

// Test 测试 MCP 服务连通性（临时连接，不缓存）
func (s *mcpService) Test(userID, id uint) (*dto.MCPTestResultResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}

	result, err := s.manager.Test(context.Background(), svc)
	if err != nil {
		return &dto.MCPTestResultResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	resp := &dto.MCPTestResultResponse{
		Success: true,
		Tools:   toToolResponses(result.Tools),
	}
	if len(result.Resources) > 0 {
		resp.Resources = toResourceResponses(result.Resources)
	}
	// 测试成功后回写工具缓存，供 Agent 构建与列表接口使用
	if len(result.Tools) > 0 {
		svc.ToolsCache = marshalJSONOrNil(result.Tools)
		if err := s.repo.Update(svc); err != nil {
			// 缓存回写失败不影响测试结果
			_ = err
		}
	}
	return resp, nil
}

// ListTools 获取 MCP 服务工具列表（优先缓存，无缓存时实时拉取）
func (s *mcpService) ListTools(userID, id uint) ([]dto.MCPToolResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}

	if len(svc.ToolsCache) > 0 {
		var cached []entity.MCPServiceTool
		if err := json.Unmarshal(svc.ToolsCache, &cached); err == nil && len(cached) > 0 {
			return toToolResponses(cached), nil
		}
	}

	cli, err := s.manager.GetClient(context.Background(), svc)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "连接 MCP 服务失败", err)
	}
	tools, err := mcp.ListTools(context.Background(), cli, mcp.BuildHeaders(svc))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "拉取 MCP 工具列表失败", err)
	}
	// 回写缓存
	if len(tools) > 0 {
		svc.ToolsCache = marshalJSONOrNil(tools)
		_ = s.repo.Update(svc)
	}
	return toToolResponses(tools), nil
}

// ListResources 获取 MCP 服务资源列表（实时拉取）
func (s *mcpService) ListResources(userID, id uint) ([]dto.MCPResourceResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}

	cli, err := s.manager.GetClient(context.Background(), svc)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "连接 MCP 服务失败", err)
	}
	resources, err := mcp.ListResources(context.Background(), cli, mcp.BuildHeaders(svc))
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "拉取 MCP 资源列表失败", err)
	}
	return toResourceResponses(resources), nil
}

// UpdateCredentials 更新服务凭据（密钥子资源）：非空字段覆盖写入，变更后强制重连
func (s *mcpService) UpdateCredentials(userID, id uint, fields map[string]string) (*dto.McpCredentialsResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}

	cfg, err := mcp.AuthConfig(svc)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "解析认证配置失败", err)
	}
	if cfg == nil {
		cfg = &entity.MCPServiceAuthConfig{}
	}

	updated := false
	if v, ok := fields["api_key"]; ok {
		cfg.APIKey = v
		updated = true
	}
	if v, ok := fields["token"]; ok {
		cfg.Token = v
		updated = true
	}
	if !updated {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "无可更新的凭据字段（支持 api_key / token）")
	}

	svc.AuthConfig = marshalJSONOrNil(cfg)
	if err := s.repo.Update(svc); err != nil {
		return nil, err
	}
	s.manager.Close(svc.ID)
	return credentialsResponse(cfg), nil
}

// DeleteCredentialField 清除指定凭据字段，变更后强制重连
func (s *mcpService) DeleteCredentialField(userID, id uint, field string) (*dto.McpCredentialsResponse, error) {
	svc, err := s.GetEntityByUser(userID, id)
	if err != nil {
		return nil, err
	}

	cfg, err := mcp.AuthConfig(svc)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "解析认证配置失败", err)
	}
	if cfg == nil {
		cfg = &entity.MCPServiceAuthConfig{}
	}

	switch field {
	case "api_key":
		cfg.APIKey = ""
	case "token":
		cfg.Token = ""
	default:
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "不支持的凭据字段（支持 api_key / token）")
	}

	svc.AuthConfig = marshalJSONOrNil(cfg)
	if err := s.repo.Update(svc); err != nil {
		return nil, err
	}
	s.manager.Close(svc.ID)
	return credentialsResponse(cfg), nil
}

// ListEnabledByUser 查询用户（含内置）已启用的 MCP 服务
func (s *mcpService) ListEnabledByUser(userID uint) ([]*entity.MCPService, error) {
	return s.repo.ListEnabledByUser(userID)
}

// GetEntityByUser 获取 MCP 服务实体
func (s *mcpService) GetEntityByUser(userID, id uint) (*entity.MCPService, error) {
	svc, err := s.repo.FindByUserAndID(userID, id)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, bizerrors.ErrResourceNotFound
	}
	return svc, nil
}

// GetToolApprovals 查询用户对指定服务的工具审批设置
func (s *mcpService) GetToolApprovals(userID, serviceID uint) ([]*dto.MCPToolApprovalResponse, error) {
	if _, err := s.GetEntityByUser(userID, serviceID); err != nil {
		return nil, err
	}
	rows, err := s.approvalRepo.ListByUserAndService(userID, serviceID)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.MCPToolApprovalResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, &dto.MCPToolApprovalResponse{
			ID:              row.ID,
			ServiceID:       row.ServiceID,
			ToolName:        row.ToolName,
			RequireApproval: row.RequireApproval,
		})
	}
	return out, nil
}

// SetToolApproval 设置（或清除）指定工具的审批要求
// 审批偏好按用户存储：对全局服务也可设置自己的偏好（不违反服务配置只读）
func (s *mcpService) SetToolApproval(userID, serviceID uint, toolName string, requireApproval bool) error {
	if _, err := s.GetEntityByUser(userID, serviceID); err != nil {
		return err
	}
	if toolName == "" {
		return bizerrors.New(bizerrors.CodeInvalidParam, "tool_name 不能为空")
	}
	return s.approvalRepo.Upsert(userID, serviceID, toolName, requireApproval)
}

// validateAuthType 校验认证类型：oauth 授权码流程 v1 未实现，明确拒绝避免"选了但实际未认证"的假象
func validateAuthType(auth *request.MCPServiceAuthConfigRequest) error {
	if auth != nil && auth.AuthType == "oauth" {
		return bizerrors.New(bizerrors.CodeInvalidParam, "OAuth 认证流程暂未支持，请使用 api_key / bearer 或留空（不认证）")
	}
	return nil
}

// validateTransport 校验传输类型必填字段
func validateTransport(transportType, url string, stdio *request.MCPServiceStdioConfigRequest) error {
	switch transportType {
	case entity.MCPTransportSSE, entity.MCPTransportHTTPStreamable:
		if strings.TrimSpace(url) == "" {
			return bizerrors.New(bizerrors.CodeInvalidParam, "该传输方式必须填写服务地址 url")
		}
	case entity.MCPTransportStdio:
		if stdio == nil || strings.TrimSpace(stdio.Command) == "" {
			return bizerrors.New(bizerrors.CodeInvalidParam, "stdio 传输必须填写启动命令")
		}
	default:
		return bizerrors.New(bizerrors.CodeInvalidParam, "不支持的传输类型，仅支持 sse / stdio / http-streamable")
	}
	return nil
}

// marshalJSONOrNil 结构体/map → entity.JSON（nil 返回 nil）
func marshalJSONOrNil(v interface{}) entity.JSON {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return entity.JSON(raw)
}

// marshalAuthConfig 认证请求 → entity.JSON（密钥一并序列化，仅落库不响应）
func marshalAuthConfig(cfg *request.MCPServiceAuthConfigRequest) entity.JSON {
	if cfg == nil {
		return nil
	}
	return marshalJSONOrNil(entity.MCPServiceAuthConfig{
		AuthType:      cfg.AuthType,
		APIKey:        cfg.APIKey,
		APIKeyHeader:  cfg.APIKeyHeader,
		Token:         cfg.Token,
		CustomHeaders: cfg.CustomHeaders,
	})
}

// toMCPServiceResponse 实体 → 响应（密钥脱敏，凭据仅标记是否已配置）
func toMCPServiceResponse(svc *entity.MCPService) *dto.MCPServiceResponse {
	resp := &dto.MCPServiceResponse{
		ID:            svc.ID,
		Name:          svc.Name,
		Description:   svc.Description,
		Enabled:       svc.Enabled,
		TransportType: svc.TransportType,
		URL:           svc.URL,
		IsBuiltin:     svc.UserID == 0,
		CreatedAt:     svc.CreatedAt,
		UpdatedAt:     svc.UpdatedAt,
	}
	if len(svc.Headers) > 0 {
		var headers map[string]string
		if json.Unmarshal(svc.Headers, &headers) == nil && len(headers) > 0 {
			resp.Headers = headers
		}
	}
	if len(svc.AdvancedConfig) > 0 {
		var cfg entity.MCPServiceAdvancedConfig
		if json.Unmarshal(svc.AdvancedConfig, &cfg) == nil {
			resp.AdvancedConfig = &dto.MCPServiceAdvancedConfigResponse{
				Timeout:    cfg.Timeout,
				RetryCount: cfg.RetryCount,
				RetryDelay: cfg.RetryDelay,
			}
		}
	}
	if len(svc.StdioConfig) > 0 {
		var cfg entity.MCPServiceStdioConfig
		if json.Unmarshal(svc.StdioConfig, &cfg) == nil {
			resp.StdioConfig = &dto.MCPServiceStdioConfigResponse{
				Command: cfg.Command,
				Args:    cfg.Args,
			}
		}
	}
	if len(svc.EnvVars) > 0 {
		var env map[string]string
		if json.Unmarshal(svc.EnvVars, &env) == nil && len(env) > 0 {
			resp.EnvVars = env
		}
	}
	if len(svc.AuthConfig) > 0 {
		var cfg entity.MCPServiceAuthConfig
		if json.Unmarshal(svc.AuthConfig, &cfg) == nil {
			resp.AuthConfig = &dto.MCPServiceAuthConfigResponse{
				AuthType:      cfg.AuthType,
				APIKeyHeader:  cfg.APIKeyHeader,
				CustomHeaders: cfg.CustomHeaders,
			}
			resp.Credentials = map[string]dto.CredentialFieldMetadata{
				"api_key": {Configured: cfg.APIKey != ""},
				"token":   {Configured: cfg.Token != ""},
			}
		}
	}
	return resp
}

// credentialsResponse 构建凭据子资源响应（标记各字段是否已配置）
func credentialsResponse(cfg *entity.MCPServiceAuthConfig) *dto.McpCredentialsResponse {
	return &dto.McpCredentialsResponse{
		Fields: map[string]dto.CredentialFieldMetadata{
			"api_key": {Configured: cfg.APIKey != ""},
			"token":   {Configured: cfg.Token != ""},
		},
	}
}

// toToolResponses 工具实体列表 → 响应列表
func toToolResponses(tools []entity.MCPServiceTool) []dto.MCPToolResponse {
	out := make([]dto.MCPToolResponse, 0, len(tools))
	for _, t := range tools {
		out = append(out, dto.MCPToolResponse{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return out
}

// toResourceResponses 资源实体列表 → 响应列表
func toResourceResponses(resources []entity.MCPServiceResource) []dto.MCPResourceResponse {
	out := make([]dto.MCPResourceResponse, 0, len(resources))
	for _, r := range resources {
		out = append(out, dto.MCPResourceResponse{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MimeType,
		})
	}
	return out
}
