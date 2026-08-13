// memorydemo 短期记忆中间件验证 demo。
//
// 验证三条主流程（不依赖数据库 / 真实模型）：
//  1. 触发压缩：消息数超过阈值 → LLM 摘要 + 保留最近 N 轮（tool 配对完整）
//  2. 不触发：消息少 → 原样透传
//  3. 降级归档：摘要模型故障 → 原文归档压缩，对话不中断
//
// 运行：go run ./cmd/memorydemo
package main

import (
	"context"
	"fmt"

	"docmind/internal/memory"
	"docmind/pkg/logger"
	"docmind/pkg/token"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// mockSummaryModel 模拟摘要模型（同单测：成功返回官方格式输出，可配置故障）
type mockSummaryModel struct {
	fail bool
}

func (m *mockSummaryModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.fail {
		return nil, fmt.Errorf("mock 摘要模型故障")
	}
	return &schema.Message{
		Role: schema.Assistant,
		Content: "<analysis>mock analysis</analysis>\n\n<summary>\n" +
			"1. 主要请求和意图：用户问题\n" +
			"6. 所有用户消息：\n<all_user_messages>\n</all_user_messages>\n" +
			"7. 待处理任务：无\n</summary>",
	}, nil
}

func (m *mockSummaryModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// buildMessages 构造 system + turns 轮对话 + 尾部工具调用链
func buildMessages(turns int) []*schema.Message {
	msgs := []*schema.Message{
		{Role: schema.System, Content: "你是一个知识库助手"},
	}
	for i := 1; i <= turns; i++ {
		msgs = append(msgs,
			&schema.Message{Role: schema.User, Content: fmt.Sprintf("第 %d 轮问题", i)},
			&schema.Message{Role: schema.Assistant, Content: fmt.Sprintf("第 %d 轮回答", i)},
		)
	}
	msgs = append(msgs,
		&schema.Message{
			Role:    schema.Assistant,
			Content: "需要查询知识库",
			ToolCalls: []schema.ToolCall{
				{ID: "call-final", Type: "function", Function: schema.FunctionCall{Name: "kb_search", Arguments: `{"query":"测试"}`}},
			},
		},
		&schema.Message{Role: schema.Tool, ToolCallID: "call-final", ToolName: "kb_search", Content: "检索结果 1"},
	)
	return msgs
}

func runScenario(name string, m model.BaseModel[*schema.Message], messages []*schema.Message) {
	fmt.Printf("\n========== 场景：%s ==========\n", name)
	fmt.Printf("压缩前消息数: %d\n", len(messages))

	mw, err := memory.NewSummaryMiddleware(context.Background(), memory.SummaryOptions{
		CreateModel: func(ctx context.Context) (model.BaseModel[*schema.Message], error) { return m, nil },
		// 触发阈值调小方便演示：消息数 > 15 即触发
		ContextMessages: 15,
		PreserveTurns:   3,
	})
	if err != nil {
		fmt.Printf("创建中间件失败: %v\n", err)
		return
	}

	state := &adk.ChatModelAgentState{Messages: messages}
	_, newState, err := mw.BeforeModelRewriteState(context.Background(), state, nil)
	if err != nil {
		fmt.Printf("中间件执行失败: %v\n", err)
		return
	}

	fmt.Printf("压缩后消息数: %d\n", len(newState.Messages))
	for i, msg := range newState.Messages {
		preview := msg.Content
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		fmt.Printf("  [%d] %s: %s\n", i, msg.Role, preview)
	}
}

func main() {
	_ = logger.InitDefault()

	fmt.Println("=== 短期记忆中间件 demo（mock 模型）===")

	// 场景 1：正常摘要压缩（12 轮 + 工具链 = 27 条 > 15）
	runScenario("正常触发 → LLM 摘要 + 保留最近 3 轮", &mockSummaryModel{}, buildMessages(12))

	// 场景 2：消息少，不触发
	runScenario("不触发 → 原样透传", &mockSummaryModel{}, buildMessages(3))

	// 场景 3：摘要模型故障 → 降级原文归档
	runScenario("摘要模型故障 → 降级归档，不中断", &mockSummaryModel{fail: true}, buildMessages(12))

	// 场景 4：增量压缩——已有旧摘要，第二次只合并增量消息（模拟第 30 轮后）
	fmt.Printf("\n========== 场景：增量压缩（旧摘要 + 增量合并） ==========\n")
	est, err := token.NewEstimator()
	if err != nil {
		fmt.Printf("创建估算器失败: %v\n", err)
		return
	}
	consolidator := memory.NewConsolidator(
		func(ctx context.Context) (model.BaseModel[*schema.Message], error) { return &mockSummaryModel{}, nil },
		est,
		200, // 触发阈值 100：旧摘要 31 + 增量 139 > 100 触发
		0,
		3, // 保底保留最近 3 轮原文
		0, // 轮数触发关闭（demo 演示 token 触发）
	)
	// 旧摘要 + 新累积的增量（5 轮 + 当前轮）
	oldSummary := "用户在第 1-20 轮讨论了 DocMind 知识库问答系统的架构设计。"
	incremental := buildIncrementalDemoMsgs(5)
	totalTokens := est.EstimateString(oldSummary) + est.EstimateMessages(incremental)
	fmt.Printf("旧摘要 token=%d + 增量 token=%d（消息 %d 条）\n",
		est.EstimateString(oldSummary), est.EstimateMessages(incremental), len(incremental))
	if consolidator.ShouldConsolidate(totalTokens, memory.CountUserTurns(incremental)) {
		newSummary, count, isRaw := consolidator.ConsolidateIncremental(context.Background(), oldSummary, incremental)
		fmt.Printf("触发压缩：%d 条增量并入摘要（降级=%v）\n", count, isRaw)
		fmt.Printf("新摘要（含旧摘要+增量合并结果）前 120 字符：\n  %s...\n", truncatePreview(newSummary, 120))
	} else {
		fmt.Println("未触发压缩")
	}
}

// buildIncrementalDemoMsgs 构造增量演示消息：rounds 轮 + 当前轮
func buildIncrementalDemoMsgs(rounds int) []*schema.Message {
	msgs := make([]*schema.Message, 0, rounds*2+1)
	for i := 21; i < 21+rounds; i++ {
		msgs = append(msgs,
			&schema.Message{Role: schema.User, Content: fmt.Sprintf("第 %d 轮问题", i)},
			&schema.Message{Role: schema.Assistant, Content: fmt.Sprintf("第 %d 轮回答", i)},
		)
	}
	msgs = append(msgs, &schema.Message{Role: schema.User, Content: "第 26 轮问题（当前轮）"})
	return msgs
}

// truncatePreview 截断字符串用于演示输出
func truncatePreview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
