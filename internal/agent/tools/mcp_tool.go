package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// prefixedTool 工具名包装器：为外部 MCP 工具加服务前缀（mcp_<service>_<tool>），
// 避免不同 MCP Server 的同名工具冲突，并兼容 AllowedTools 白名单按名过滤
// requireApproval=true 时调用前拦截（用户对高风险工具设置的人工审批偏好）
type prefixedTool struct {
	inner           tool.BaseTool
	prefix          string
	requireApproval bool
}

// Info 返回改名后的工具元数据
func (p *prefixedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	info, err := p.inner.Info(ctx)
	if err != nil {
		return nil, err
	}
	info.Name = p.prefix + "_" + info.Name
	return info, nil
}

// InvokableRun 转发工具调用（含人工审批拦截）
func (p *prefixedTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if p.requireApproval {
		info, err := p.inner.Info(ctx)
		if err != nil {
			return "", fmt.Errorf("MCP 工具审批检查失败: %w", err)
		}
		return "", fmt.Errorf("工具 %s 需要人工审批：请在 MCP 设置页将该工具标记为不需要审批后重试", info.Name)
	}
	inv, ok := p.inner.(tool.InvokableTool)
	if !ok {
		info, err := p.inner.Info(ctx)
		if err != nil {
			return "", fmt.Errorf("MCP 工具不支持执行: %w", err)
		}
		return "", fmt.Errorf("MCP 工具 %s 不支持 InvokableRun", info.Name)
	}
	return inv.InvokableRun(ctx, argumentsInJSON, opts...)
}

// sanitizeToolName 清洗服务名为合法工具名前缀（小写，非字母数字转为下划线）
func sanitizeToolName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
