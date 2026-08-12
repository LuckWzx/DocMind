package sandbox

import (
	"context"
	"time"
)

// Sandbox 沙箱接口：数据计算工具（python_exec）依赖的执行契约。
// 阶段二接口契约：与实现无关，未来可无缝切换 Docker 沙箱等强隔离实现。
type Sandbox interface {
	// Execute 执行一段 Python 代码，返回结构化结果。
	Execute(ctx context.Context, code string) (*SandboxResult, error)
}

// FileSandbox 扩展接口（可选）：支持挂载数据文件到沙箱工作目录。
// 数据计算场景（knowledge_id 指向 CSV/Excel）由 python_exec 工具断言使用。
type FileSandbox interface {
	ExecuteWithFiles(ctx context.Context, code string, files map[string]string) (*SandboxResult, error)
}

// SandboxEvent 单条结构化结果事件（沙箱输出协议）
type SandboxEvent struct {
	Type       string     `json:"type"`                  // text / table / chart / error
	Content    string     `json:"content,omitempty"`     // text / error 内容
	Columns    []string   `json:"columns,omitempty"`     // table 列名
	Rows       [][]string `json:"rows,omitempty"`        // table 行数据
	Format     string     `json:"format,omitempty"`      // chart 格式（png）
	DataBase64 string     `json:"data_base64,omitempty"` // chart base64 数据
}

// SandboxResult 单次执行结果
type SandboxResult struct {
	Stdout   string         // 用户代码 print 输出（协议标记之前的文本）
	Stderr   string         // 错误输出（traceback 等）
	Events   []SandboxEvent // 结构化结果事件
	Duration time.Duration  // 执行耗时
}
