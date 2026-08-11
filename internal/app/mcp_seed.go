package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"docmind/internal/mcp"
	"docmind/internal/model/entity"
	"docmind/pkg/config"
	"docmind/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// seedPresetMCPServices 将配置文件中声明的全局（系统级）MCP 服务预置到数据库（user_id=0）
// 语义：系统安装的 MCP 服务，所有用户可见、可挂载到 Agent，但只读（不可修改/删除）
// 同步策略：
//   - 按 user_id=0 + name 匹配，不存在则创建，存在则更新连接配置（配置文件是唯一事实源）
//   - 数据库中存在但配置中已删除的系统级服务 → 移除（仅移除 user_id=0，不影响用户自建）
func seedPresetMCPServices(db *gorm.DB, presets []config.MCPServicePresetConfig, manager *mcp.Manager) error {
	if len(presets) == 0 {
		return nil
	}
	logger.Info("开始预置全局 MCP 服务", zap.Int("count", len(presets)))

	keepNames := make(map[string]struct{}, len(presets))
	for i := range presets {
		p := &presets[i]
		if err := validatePreset(p); err != nil {
			return fmt.Errorf("预置 MCP 服务 %q 配置错误: %w", p.Name, err)
		}
		keepNames[p.Name] = struct{}{}

		svc, err := findPresetByName(db, p.Name)
		if err != nil {
			return err
		}
		built := buildPresetEntity(p)
		if svc == nil {
			if err := db.Create(built).Error; err != nil {
				return fmt.Errorf("创建全局 MCP 服务 %q 失败: %w", p.Name, err)
			}
			logger.Info("已预置全局 MCP 服务", zap.String("name", p.Name))
		} else {
			built.ID = svc.ID
			built.CreatedAt = svc.CreatedAt
			if err := db.Save(built).Error; err != nil {
				return fmt.Errorf("更新全局 MCP 服务 %q 失败: %w", p.Name, err)
			}
			// 连接参数可能已变更，断开缓存连接强制重建
			manager.Close(svc.ID)
			logger.Info("已更新全局 MCP 服务", zap.String("name", p.Name))
		}
	}

	// 移除配置中已删除的系统级服务（仅 user_id=0）
	var globals []*entity.MCPService
	if err := db.Where("user_id = 0").Find(&globals).Error; err != nil {
		return err
	}
	for _, g := range globals {
		if _, ok := keepNames[g.Name]; ok {
			continue
		}
		manager.Close(g.ID)
		if err := db.Delete(&entity.MCPService{}, g.ID).Error; err != nil {
			return fmt.Errorf("移除已删除的全局 MCP 服务 %q 失败: %w", g.Name, err)
		}
		logger.Info("已移除配置中不存在的全局 MCP 服务", zap.String("name", g.Name))
	}
	return nil
}

// validatePreset 校验预置服务必填字段
func validatePreset(p *config.MCPServicePresetConfig) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("缺少 name")
	}
	switch p.TransportType {
	case entity.MCPTransportSSE, entity.MCPTransportHTTPStreamable:
		if strings.TrimSpace(p.URL) == "" {
			return fmt.Errorf("传输类型 %s 缺少 url", p.TransportType)
		}
	case entity.MCPTransportStdio:
		if strings.TrimSpace(p.Command) == "" {
			return fmt.Errorf("传输类型 stdio 缺少 command")
		}
	default:
		return fmt.Errorf("不支持的传输类型 %q", p.TransportType)
	}
	return nil
}

// buildPresetEntity 将预置配置转为 MCPService 实体（user_id=0 = 系统级）
func buildPresetEntity(p *config.MCPServicePresetConfig) *entity.MCPService {
	svc := &entity.MCPService{
		UserID:        0,
		Name:          p.Name,
		Description:   p.Description,
		TransportType: p.TransportType,
		URL:           p.URL,
		Enabled:       true,
	}
	if p.Enabled != nil {
		svc.Enabled = *p.Enabled
	}
	if len(p.Headers) > 0 {
		svc.Headers, _ = json.Marshal(p.Headers)
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	svc.AdvancedConfig, _ = json.Marshal(&entity.MCPServiceAdvancedConfig{
		Timeout:    timeout,
		RetryCount: 1,
		RetryDelay: 1,
	})
	if p.TransportType == entity.MCPTransportStdio && p.Command != "" {
		svc.StdioConfig, _ = json.Marshal(&entity.MCPServiceStdioConfig{
			Command: p.Command,
			Args:    p.Args,
		})
	}
	if len(p.Env) > 0 {
		env := make(map[string]string, len(p.Env))
		for _, kv := range p.Env {
			if idx := strings.IndexByte(kv, '='); idx > 0 {
				env[kv[:idx]] = kv[idx+1:]
			}
		}
		if len(env) > 0 {
			svc.EnvVars, _ = json.Marshal(env)
		}
	}
	return svc
}

// findPresetByName 按 user_id=0 + name 查询已预置的全局服务
func findPresetByName(db *gorm.DB, name string) (*entity.MCPService, error) {
	var svc entity.MCPService
	err := db.Where("user_id = 0 AND name = ?", name).First(&svc).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &svc, nil
}
