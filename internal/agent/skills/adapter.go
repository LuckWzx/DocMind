package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
)

// DefaultSkillsDir 默认技能目录（相对运行目录）
const DefaultSkillsDir = "configs/skills"

// whitelistBackend 白名单包装：仅暴露 SelectedSkills 中的技能
// 供 LoadSkillMiddleware 使用，保证工具描述与执行都只面向白名单内技能
type whitelistBackend struct {
	inner skill.Backend
	allow map[string]struct{}
}

// List 返回白名单内的技能元数据
func (w *whitelistBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	matters, err := w.inner.List(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]skill.FrontMatter, 0, len(matters))
	for _, m := range matters {
		if _, ok := w.allow[m.Name]; ok {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// Get 白名单外的技能视为不存在
func (w *whitelistBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	if _, ok := w.allow[name]; !ok {
		return skill.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	return w.inner.Get(ctx, name)
}

// LoadSkillMiddleware 加载技能目录并构造 skill middleware（挂到 ChatModelAgentConfig.Handlers）
// selected 为 nil 时加载全部技能；非 nil 时按白名单过滤（空切片 = 全部禁用）
func LoadSkillMiddleware(ctx context.Context, baseDir string, selected []string) (adk.ChatModelAgentMiddleware, error) {
	backend, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
		Backend: NewLocalBackend(),
		BaseDir: baseDir,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化技能后端失败: %w", err)
	}
	if selected != nil {
		allow := make(map[string]struct{}, len(selected))
		for _, name := range selected {
			allow[name] = struct{}{}
		}
		backend = &whitelistBackend{inner: backend, allow: allow}
	}
	handler, err := skill.NewMiddleware(ctx, &skill.Config{Backend: backend})
	if err != nil {
		return nil, fmt.Errorf("创建技能中间件失败: %w", err)
	}
	return handler, nil
}
