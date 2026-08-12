package tools

import (
	"context"
	"fmt"

	"docmind/internal/repository"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// DataSchemaArgs data_schema 工具参数（模型按 JSON Schema 填充）
type DataSchemaArgs struct {
	KnowledgeID string `json:"knowledge_id"`
}

// NewDataSchemaTool 构建数据表结构查看工具：
// 加载 CSV/Excel 并返回表结构（表名、列、类型、行数），不执行用户 SQL。
// 与 data_analysis 共享同一请求级 DuckDB 会话：模型可先用本工具了解结构，
// 再写 SQL 查询（表已加载，data_analysis 不会重复加载）。
func NewDataSchemaTool(
	knowledgeRepo repository.KnowledgeRepository,
	kbRepo repository.KnowledgeBaseRepository,
	userID uint,
	kbScope []string,
	session *analysisSession,
) (tool.BaseTool, error) {
	schemaFn := func(ctx context.Context, args DataSchemaArgs) (string, error) {
		knowledge, tableName, err := resolveAnalysisKnowledge(ctx, knowledgeRepo, kbRepo, userID, kbScope, args.KnowledgeID)
		if err != nil {
			return fmt.Sprintf("获取表结构失败：%v。请确认 knowledge_id 为知识库中 CSV/Excel 文件的条目 ID。", err), nil
		}
		if isAnalysisRemoteURL(knowledge.FileURL) {
			return "获取表结构失败：远程存储的文件暂不支持，请使用本地存储的知识库。", nil
		}
		info, err := session.loadTable(ctx, knowledge.FileURL, knowledge.FileType, tableName)
		if err != nil {
			return fmt.Sprintf("获取表结构失败：%v。请确认文件为 UTF-8/GBK 编码的 CSV 或 .xlsx 格式。", err), nil
		}
		return formatAnalysisSchema(info), nil
	}

	return utils.InferTool[DataSchemaArgs, string](
		"data_schema",
		"查看知识库中表格文件（CSV/Excel）的数据表结构（表名、列名、类型、行数）。\n"+
			"## 使用方式\n"+
			"1. 参数 knowledge_id 为表格文件对应知识条目的 ID（先调用 kb_search 检索该表格内容，从检索结果的 knowledge_id 字段获取）；\n"+
			"2. 返回的表结构可直接用于 data_analysis 工具编写 SQL；\n"+
			"3. 与 data_analysis 共享同一轮对话内的已加载表，不会重复加载文件。\n"+
			"## 注意事项\n"+
			"- 仅查看结构，不执行任何 SQL；\n"+
			"- 若文件较大，加载可能需要数秒。",
		schemaFn,
	)
}
