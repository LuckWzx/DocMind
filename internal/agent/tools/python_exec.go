package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docmind/internal/repository"
	"docmind/internal/sandbox"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"
)

// sandboxChartDir 沙箱图表落盘目录（相对项目根）。
// 通过 /files?file_path=local://sandbox/<name> 访问（见 internal/api/v1/files），
// 与 knowledge 默认存储根 data/files 保持一致；URL 形如 local://sandbox/<uuid>.png
const sandboxChartDir = "data/files/sandbox"

// sandboxChartURLPrefix 图表 local:// 协议前缀（files 控制器按此解析）
const sandboxChartURLPrefix = "local://sandbox/"

// sandboxChartTTL 图表文件保留时长：超过后下次执行时清理
const sandboxChartTTL = 24 * time.Hour

// PythonExecArgs python_exec 工具参数（模型按 JSON Schema 填充）
type PythonExecArgs struct {
	KnowledgeID string `json:"knowledge_id"` // 可选：要分析的数据文件（CSV/Excel 知识条目 ID）
	Code        string `json:"code"`         // 要执行的 Python 代码
}

// NewPythonExecTool 构建 Python 代码执行工具（数据计算工具）：
// 模型生成 Python 代码 → 沙箱隔离执行 → 返回结构化结果（文本/表格/图表）。
// 沙箱内可用：duckdb（SQL 分析）、pandas、matplotlib（图表）、标准库；
// 提供 emit_text / emit_table / emit_chart 三个结果输出 API。
func NewPythonExecTool(
	sb sandbox.Sandbox,
	knowledgeRepo repository.KnowledgeRepository,
	kbRepo repository.KnowledgeBaseRepository,
	userID uint,
	kbScope []string,
) (tool.BaseTool, error) {
	execFn := func(ctx context.Context, args PythonExecArgs) (string, error) {
		code := strings.TrimSpace(args.Code)
		if code == "" {
			return "代码为空。请提供要执行的 Python 代码。", nil
		}

		// 数据文件挂载：knowledge_id → 沙箱工作目录（Python 侧用文件名相对路径读取）
		var files map[string]string
		if args.KnowledgeID != "" {
			if knowledgeRepo == nil {
				return "当前未启用知识库数据文件挂载能力，请直接使用代码处理传入数据。", nil
			}
			knowledge, _, err := resolveAnalysisKnowledge(ctx, knowledgeRepo, kbRepo, userID, kbScope, args.KnowledgeID)
			if err != nil {
				return fmt.Sprintf("执行失败：%v。请确认 knowledge_id 为知识库中 CSV/Excel 文件的条目 ID。", err), nil
			}
			if isAnalysisRemoteURL(knowledge.FileURL) {
				return "执行失败：远程存储的文件暂不支持，请使用本地存储的知识库。", nil
			}
			files = map[string]string{filepath.Base(knowledge.FileURL): knowledge.FileURL}
		}

		// 执行（优先走带文件挂载的扩展接口）
		fsb, ok := sb.(sandbox.FileSandbox)
		var result *sandbox.SandboxResult
		var err error
		if ok {
			result, err = fsb.ExecuteWithFiles(ctx, code, files)
		} else {
			result, err = sb.Execute(ctx, code)
		}
		if err != nil {
			return fmt.Sprintf("代码执行失败：%v。请检查代码语法与逻辑后重试。", err), nil
		}

		// 图表事件落盘：base64 → data/files/sandbox/<uuid>.png，返回 local:// 协议 URL
		// （前端经 /files 代理 + token fetch 渲染，见 internal/api/v1/files）
		for i, ev := range result.Events {
			if ev.Type != "chart" || ev.DataBase64 == "" {
				continue
			}
			url, storeErr := storeSandboxChart(ev.DataBase64)
			if storeErr != nil {
				continue
			}
			result.Events[i] = sandbox.SandboxEvent{
				Type:    "text",
				Content: fmt.Sprintf("图表已生成：![季度销售趋势图](%s)", url),
			}
		}
		return formatSandboxResult(result, files), nil
	}

	return utils.InferTool[PythonExecArgs, string](
		"python_exec",
		"执行 Python 代码进行数据分析计算。当需要 SQL 无法完成的复杂分析（时间序列趋势、环比/同比计算、pandas 多步数据处理、生成图表）时必须调用本工具。\n"+
			"## 使用方式\n"+
			"1. 参数 knowledge_id 为数据文件对应知识条目的 ID（先调用 kb_search 检索表格内容获取）；文件会挂载到沙箱工作目录，代码中用文件名（如 data.csv）相对路径读取；\n"+
			"2. 参数 code 为要执行的 Python 代码；沙箱预装 duckdb、pandas、matplotlib 及标准库，禁网络/禁子进程/禁读写工作目录外文件；\n"+
			"3. 结果输出：print() 输出到 stdout；结构化结果调用 emit_text(对象)、emit_table(DataFrame)、emit_chart(Figure) 返回；\n"+
			"4. 图表（emit_chart）会自动保存为图片文件并返回 Markdown 图片引用（local:// 协议），最终回答中必须原样包含该图片引用，不要省略或改写；\n"+
			"5. 读表格文件推荐 duckdb.read_csv('文件名') / duckdb.read_excel('文件名') 或 pandas.read_csv；\n"+
			"6. 分析完成后在最终回答中总结结论。\n"+
			"## 注意事项\n"+
			"- 代码执行有超时限制（默认 30 秒），大数据集请控制计算量；\n"+
			"- 禁止访问网络、子进程、工作目录外的文件；\n"+
			"- 图表请使用 matplotlib 的 Figure 对象传给 emit_chart。",
		execFn,
	)
}

// formatSandboxResult 沙箱结构化结果 → 模型可读文本
func formatSandboxResult(r *sandbox.SandboxResult, files map[string]string) string {
	var b strings.Builder
	b.WriteString("=== Python 执行结果 ===\n")
	if len(files) > 0 {
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		b.WriteString(fmt.Sprintf("已挂载数据文件：%s（代码中用相对路径读取）\n", strings.Join(names, ", ")))
	}
	if r.Stdout != "" {
		b.WriteString("--- 标准输出 ---\n" + r.Stdout + "\n")
	}
	for _, ev := range r.Events {
		switch ev.Type {
		case "text":
			b.WriteString(ev.Content + "\n")
		case "table":
			b.WriteString("--- 数据表 ---\n" + renderSandboxTable(ev) + "\n")
		case "chart":
			b.WriteString(fmt.Sprintf("--- 图表 ---\n已生成 %s 图表（base64 约 %d KB），在最终回答中描述图表结论\n",
				ev.Format, len(ev.DataBase64)/1024))
		case "error":
			b.WriteString("--- 执行出错（请修正代码后重试）---\n" + ev.Content + "\n")
		}
	}
	if r.Stderr != "" {
		b.WriteString("--- 标准错误 ---\n" + r.Stderr + "\n")
	}
	b.WriteString(fmt.Sprintf("耗时：%.1fs\n", r.Duration.Seconds()))
	return b.String()
}

// renderSandboxTable 事件 → Markdown 表格
func renderSandboxTable(ev sandbox.SandboxEvent) string {
	var b strings.Builder
	if len(ev.Columns) > 0 {
		b.WriteString("| " + strings.Join(ev.Columns, " | ") + " |\n")
		b.WriteString("|" + strings.Repeat("---|", len(ev.Columns)) + "\n")
	}
	for _, row := range ev.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = strings.ReplaceAll(v, "|", "\\|")
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	return b.String()
}

// storeSandboxChart 图表 base64 → data/files/sandbox/<uuid>.png，返回 local:// 协议 URL。
// 顺带清理超过保留时长的旧图表（轻量扫描，文件数少，开销可忽略）。
func storeSandboxChart(dataBase64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(dataBase64)
	if err != nil {
		return "", fmt.Errorf("解码图表数据失败: %w", err)
	}
	if len(raw) == 0 || len(raw) > 10<<20 {
		return "", fmt.Errorf("图表数据大小非法: %d 字节", len(raw))
	}

	if err := os.MkdirAll(sandboxChartDir, 0o755); err != nil {
		return "", fmt.Errorf("创建图表目录失败: %w", err)
	}
	name := uuid.NewString() + ".png"
	if err := os.WriteFile(filepath.Join(sandboxChartDir, name), raw, 0o644); err != nil {
		return "", fmt.Errorf("写入图表文件失败: %w", err)
	}

	cleanupSandboxCharts()
	return sandboxChartURLPrefix + name, nil
}

// cleanupSandboxCharts 删除超过保留时长的旧图表文件（幂等，错误忽略）
func cleanupSandboxCharts() {
	entries, err := os.ReadDir(sandboxChartDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-sandboxChartTTL)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(sandboxChartDir, e.Name()))
		}
	}
}
