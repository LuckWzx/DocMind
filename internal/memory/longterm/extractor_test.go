package longterm

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"docmind/pkg/logger"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TestMain 初始化日志（logger.Warnf 等无 nil 保护，测试环境必须先初始化）
func TestMain(m *testing.M) {
	_ = logger.InitDefault()
	os.Exit(m.Run())
}

// mockModel 模拟 ToolCallingChatModel（按调用顺序返回预设内容，支持前置失败）
type mockModel struct {
	responses []string
	index     int
	failCount int // 前 N 次调用返回错误（模拟重试路径）
	err       error
}

func (m *mockModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.index < m.failCount {
		m.index++
		return nil, m.err
	}
	// 成功响应从 responses[0] 开始取（重试成功后继续消费后续响应）
	respIdx := m.index - m.failCount
	if respIdx >= len(m.responses) {
		return nil, errors.New("mock: 响应耗尽")
	}
	content := m.responses[respIdx]
	m.index++
	return &schema.Message{Role: schema.Assistant, Content: content}, nil
}

func (m *mockModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("mock: 未实现 Stream")
}

func (m *mockModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func newTestExtractor(m *mockModel) *GraphExtractor {
	return NewGraphExtractor(func(ctx context.Context, modelID string) (model.ToolCallingChatModel, error) { return m, nil })
}

const validGraphJSON = `{
	"summary": "用户讨论了 DocMind 项目的知识库问答系统架构。",
	"entities": [
		{"title": "DocMind", "type": "Project", "description": "知识库问答系统"},
		{"title": "Neo4j", "type": "Technology", "description": "图数据库"}
	],
	"relationships": [
		{"source": "DocMind", "target": "Neo4j", "description": "使用", "weight": 0.9}
	]
}`

func TestExtractGraph_Success(t *testing.T) {
	m := &mockModel{responses: []string{validGraphJSON}}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "User: 你好\nAssistant: 你好")
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if result.Summary == "" {
		t.Error("摘要为空")
	}
	if len(result.Entities) != 2 {
		t.Errorf("实体数量 = %d, 期望 2", len(result.Entities))
	}
	if len(result.Relationships) != 1 {
		t.Errorf("关系数量 = %d, 期望 1", len(result.Relationships))
	}
	if result.Relationships[0].Source != "DocMind" || result.Relationships[0].Target != "Neo4j" {
		t.Errorf("关系端点错误: %s -> %s", result.Relationships[0].Source, result.Relationships[0].Target)
	}
}

func TestExtractGraph_RetryAfterFailure(t *testing.T) {
	// 第 1 次调用失败，第 2 次成功（重试路径）
	m := &mockModel{responses: []string{validGraphJSON}, failCount: 1, err: errors.New("LLM 超时")}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "对话")
	if err != nil {
		t.Fatalf("重试成功后失败: %v", err)
	}
	if len(result.Entities) == 0 {
		t.Error("重试成功后实体为空")
	}
}

func TestExtractGraph_AllFailures(t *testing.T) {
	// 全部调用失败 → 返回 error（上层降级跳过）
	m := &mockModel{responses: []string{}, failCount: 99, err: errors.New("LLM 不可用")}
	extractor := newTestExtractor(m)

	_, err := extractor.ExtractGraph(context.Background(), "1", "对话")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestExtractGraph_ParseRetry(t *testing.T) {
	// 第 1 次返回非 JSON 文本，第 2 次返回合法 JSON
	m := &mockModel{responses: []string{"抱歉，我无法处理", validGraphJSON}}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "对话")
	if err != nil {
		t.Fatalf("解析重试后仍失败: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Errorf("实体数量 = %d, 期望 2", len(result.Entities))
	}
}

func TestExtractGraph_CodeBlockWrapped(t *testing.T) {
	// 模型用 ```json 代码块包裹输出也能解析
	wrapped := "```json\n" + validGraphJSON + "\n```"
	m := &mockModel{responses: []string{wrapped}}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "对话")
	if err != nil {
		t.Fatalf("代码块包裹解析失败: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Errorf("实体数量 = %d, 期望 2", len(result.Entities))
	}
}

func TestExtractGraph_EmptyEntities(t *testing.T) {
	// 空实体：仅摘要保留（Episode 仍可被检索）
	emptyJSON := `{"summary": "简单寒暄", "entities": [], "relationships": []}`
	m := &mockModel{responses: []string{emptyJSON}}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "User: 你好\nAssistant: 你好")
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if result.Summary != "简单寒暄" {
		t.Errorf("摘要 = %q, 期望 简单寒暄", result.Summary)
	}
	if len(result.Entities) != 0 || len(result.Relationships) != 0 {
		t.Error("空实体场景应返回空列表")
	}
}

func TestExtractGraph_DirtyDataFiltered(t *testing.T) {
	// 空白实体名 / 空白关系端点被剔除
	dirtyJSON := `{
		"summary": "s",
		"entities": [{"title": "  ", "type": "X", "description": ""}, {"title": "有效实体", "type": "Concept", "description": ""}],
		"relationships": [{"source": "", "target": "有效实体", "description": "", "weight": 0.5}]
	}`
	m := &mockModel{responses: []string{dirtyJSON}}
	extractor := newTestExtractor(m)

	result, err := extractor.ExtractGraph(context.Background(), "1", "对话")
	if err != nil {
		t.Fatalf("提取失败: %v", err)
	}
	if len(result.Entities) != 1 || result.Entities[0].Title != "有效实体" {
		t.Errorf("脏实体未过滤: %+v", result.Entities)
	}
	if len(result.Relationships) != 0 {
		t.Errorf("脏关系未过滤: %+v", result.Relationships)
	}
}

func TestExtractKeywords_Success(t *testing.T) {
	m := &mockModel{responses: []string{`{"keywords": ["DocMind", "知识库"]}`}}
	extractor := newTestExtractor(m)

	keywords, err := extractor.ExtractKeywords(context.Background(), "1", "上次讨论的 DocMind 是什么")
	if err != nil {
		t.Fatalf("关键词提取失败: %v", err)
	}
	if len(keywords) != 2 || keywords[0] != "DocMind" || keywords[1] != "知识库" {
		t.Errorf("关键词 = %v, 期望 [DocMind 知识库]", keywords)
	}
}

func TestExtractKeywords_Failure(t *testing.T) {
	m := &mockModel{responses: []string{}, failCount: 99, err: errors.New("LLM 不可用")}
	extractor := newTestExtractor(m)

	_, err := extractor.ExtractKeywords(context.Background(), "1", "查询")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestExtractKeywords_WhitespaceFiltered(t *testing.T) {
	m := &mockModel{responses: []string{`{"keywords": ["  DocMind  ", "  ", "知识库"]}`}}
	extractor := newTestExtractor(m)

	keywords, err := extractor.ExtractKeywords(context.Background(), "1", "查询")
	if err != nil {
		t.Fatalf("关键词提取失败: %v", err)
	}
	if len(keywords) != 2 {
		t.Errorf("关键词数量 = %d, 期望 2（空白应过滤）: %v", len(keywords), keywords)
	}
	for _, k := range keywords {
		if strings.TrimSpace(k) != k {
			t.Errorf("关键词未去空白: %q", k)
		}
	}
}
