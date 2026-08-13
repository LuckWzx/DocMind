package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"docmind/pkg/logger"
	"docmind/pkg/token"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	// maxConsolidationAttempts LLM 摘要最大尝试次数，超过后降级为原文归档
	maxConsolidationAttempts = 3
	// consolidationTimeout 单次摘要生成的超时
	consolidationTimeout = 60 * time.Second
	// consolidationTargetRatio 压缩目标：触发阈值 × 60%（为摘要消息与后续增量留出余量）
	consolidationTargetRatio = 0.6
	// summaryReserveTokens 为摘要消息预留的 Token 预算
	summaryReserveTokens = 500
	// consolidationPromptMaxTokens 摘要生成的最大输出 Token
	consolidationPromptMaxTokens = 2000
	// consolidationTemperature 摘要生成温度（低温度保证事实性）
	consolidationTemperature = 0.3

	// promptUserMaxChars 摘要 prompt 中用户消息截断长度
	promptUserMaxChars = 2000
	// promptAssistantMaxChars 摘要 prompt 中 assistant 消息截断长度
	promptAssistantMaxChars = 2000
	// promptToolCallMaxChars 摘要 prompt 中带工具调用的 assistant 消息截断长度
	promptToolCallMaxChars = 1000
	// promptToolMaxChars 摘要 prompt 中工具结果截断长度
	promptToolMaxChars = 1000
)

// Consolidator 压缩 pipeline（quick-answer）模式的对话历史。
//
// pipeline 是 compose.Graph，无法挂载 ADK 中间件，因此在 chat_service
// 加载历史之后、构建 pipeline.Context 之前手动调用：
//
//	if consolidator.ShouldConsolidate(estimator.EstimateMessages(history)) {
//	    history, _ = consolidator.Consolidate(ctx, history)
//	}
//
// 压缩规则（与阶段二.md 契约一致）：
//   - 保留：system prompt + 当前轮（最后 user 及其后全部）+ 最近 preserveTurns 轮原文（保底）+ 预算内更多历史
//   - 压缩：其余历史 → LLM 摘要为一条 system 消息
//   - tool_call 边界保护：从历史尾部回扫时按"tool 组 + 前驱 assistant"整体保留
//   - 降级：LLM 摘要失败 3 次 → rawArchive 原文归档
type Consolidator struct {
	createModel    func(ctx context.Context) (model.BaseModel[*schema.Message], error)
	estimator      *token.Estimator
	maxTokens      int     // 上下文窗口
	threshold      float64 // 触发比例（0-1，默认 0.5）
	preserveTurns  int     // 压缩时保底保留的最近完整轮数（<=0 用默认 5）
	turnsThreshold int     // 增量轮数触发阈值（>0 生效：增量轮数达到即触发，与 token 阈值任一满足）

	mu       sync.Mutex
	model    model.BaseModel[*schema.Message]
	modelErr error
}

// NewConsolidator 创建压缩器。
// createModel 为摘要模型工厂（懒创建 + 缓存，避免未触发压缩时产生模型开销）；
// maxTokens 为上下文窗口 Token 数；threshold 为触发比例（0-1，0 使用默认 0.5）；
// preserveTurns 为压缩时保底保留的最近完整轮数（<=0 使用默认 5），
// 与 Agent 模式中间件的 PreserveTurns 语义一致；
// turnsThreshold 为增量轮数触发阈值（<=0 退化为纯 token 触发，
// 推荐用 TurnsThresholdForWindow 按模型上下文分档）。
func NewConsolidator(
	createModel func(ctx context.Context) (model.BaseModel[*schema.Message], error),
	estimator *token.Estimator,
	maxTokens int,
	threshold float64,
	preserveTurns int,
	turnsThreshold int,
) *Consolidator {
	if threshold <= 0 || threshold >= 1 {
		threshold = DefaultConsolidationThreshold
	}
	if preserveTurns <= 0 {
		preserveTurns = DefaultPreserveTurns
	}
	return &Consolidator{
		createModel:    createModel,
		estimator:      estimator,
		maxTokens:      maxTokens,
		threshold:      threshold,
		preserveTurns:  preserveTurns,
		turnsThreshold: turnsThreshold,
	}
}

// ShouldConsolidate 判断是否达到压缩触发条件：Token 超阈值（maxTokens × threshold）
// 或增量轮数达 turnsThreshold（>0 时生效），任一满足即触发。
// 轮数触发解决低 token 密度对话长期不触发的问题；token 阈值兜底防爆窗。
func (c *Consolidator) ShouldConsolidate(currentTokens int, currentTurns int) bool {
	if c.maxTokens <= 0 {
		return false
	}
	triggerAt := int(float64(c.maxTokens) * c.threshold)
	if currentTokens > triggerAt {
		return true
	}
	return c.turnsThreshold > 0 && currentTurns >= c.turnsThreshold
}

// Consolidate 压缩对话历史：将可压缩部分替换为一条摘要 system 消息。
// 输入为调用方加载的历史消息（不含当前轮；若包含当前轮也能正确识别）。
func (c *Consolidator) Consolidate(ctx context.Context, messages []*schema.Message) ([]*schema.Message, error) {
	if len(messages) <= 3 {
		return messages, nil
	}

	// 1. 拆分开头的 system 消息与对话消息
	systemMsgs, contextMsgs := splitSystemMsgs(messages)

	// 2. 找最后一条 user 消息：它及其后全部属于"当前轮"，原样保留
	lastUserIdx := -1
	for i := len(contextMsgs) - 1; i >= 0; i-- {
		if contextMsgs[i].Role == schema.User {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx <= 0 {
		// 没有可压缩的历史（当前轮就是全部）
		return messages, nil
	}

	history := contextMsgs[:lastUserIdx]
	tail := contextMsgs[lastUserIdx:]
	if len(history) < 2 {
		return messages, nil
	}

	// 3. 计算保留预算：压缩目标 = 触发阈值 × 60%，减去 system、当前轮与摘要预留；
	//    再叠加保底保留最近 preserveTurns 轮原文（用户显式配置）
	targetTokens := int(float64(c.maxTokens) * c.threshold * consolidationTargetRatio)
	tailTokens := c.estimator.EstimateMessages(tail)
	keepFromEnd := c.findKeepBoundary(history, targetTokens, systemMsgs, tailTokens)
	keepFromEnd = c.applyPreserveTurns(keepFromEnd, history, tail)

	// 4. 预算足够保留全部历史 → 无需压缩
	if keepFromEnd >= len(history) {
		return messages, nil
	}

	toConsolidate := history[:len(history)-keepFromEnd]
	toKeep := history[len(history)-keepFromEnd:]

	// 5. LLM 摘要，失败后降级为原文归档
	summary, err := c.summarizeWithRetry(ctx, toConsolidate)
	if err != nil {
		logger.Warnf("[MemoryConsolidator] LLM 摘要失败，降级为原文归档: %v", err)
		summary = c.rawArchiveSummary(toConsolidate)
	}

	summaryMsg := &schema.Message{
		Role: schema.System,
		Content: fmt.Sprintf("[Memory Summary - %d earlier messages consolidated]\n\n%s",
			len(toConsolidate), summary),
	}

	result := make([]*schema.Message, 0, len(systemMsgs)+1+len(toKeep)+len(tail))
	result = append(result, systemMsgs...)
	result = append(result, summaryMsg)
	result = append(result, toKeep...)
	result = append(result, tail...)

	logger.Infof("[MemoryConsolidator] 压缩 %d 条消息 → 摘要，保留 %d 条历史 + 当前轮 %d 条",
		len(toConsolidate), len(toKeep), len(tail))
	return result, nil
}

// ConsolidateIncremental 增量压缩：将持久化的旧摘要与压缩边界后的增量消息
// 合并为一份新摘要（预算驱动：预算足够保留全部历史时不压缩，原文优于摘要）。
//
// 与 Consolidate（全量重算）的区别：LLM 的输入只有"旧摘要 + 增量消息"，
// 压缩成本与历史总长度无关，只与增量大小有关——历史越长收益越大。
//
// 入参：
//   - oldSummary：已持久化的旧摘要正文（可为空字符串，等价于首次压缩）
//   - incremental：压缩边界之后的全部消息（含当前轮；当前轮原样保留不压缩）
//
// 返回新摘要正文（由调用方持久化并包装为 system 消息）、本次并入摘要的消息条数，
// 以及是否降级为原文归档（LLM 摘要失败时降级为"旧摘要 + 原文归档"，对话不中断）。
func (c *Consolidator) ConsolidateIncremental(ctx context.Context, oldSummary string, incremental []*schema.Message) (newSummary string, consolidatedCount int, isRaw bool) {
	return c.consolidateIncremental(ctx, oldSummary, incremental, false, 0)
}

// ConsolidateIncrementalForced 强制增量压缩（手动压缩语义）：保留最近 preserveTurns
// 轮原文（含当前轮），其余增量全部并入摘要——不按"预算足够就全保留"的优化逻辑，
// 用户主动操作优先于摘要损耗权衡；token 预算仅作防爆窗兜底（保底轮数超窗时
// 退化为预算驱动）。返回与 ConsolidateIncremental 一致。
func (c *Consolidator) ConsolidateIncrementalForced(ctx context.Context, oldSummary string, incremental []*schema.Message, preserveTurns int) (newSummary string, consolidatedCount int, isRaw bool) {
	return c.consolidateIncremental(ctx, oldSummary, incremental, true, preserveTurns)
}

// consolidateIncremental 增量压缩核心实现。forced=true 时保留策略反转：
// 自动模式 = max(预算保留, 保底保留)（尽量多保留原文）；
// 强制模式 = min(预算保留, 保底保留)（尽量多压缩，预算防爆窗）。
func (c *Consolidator) consolidateIncremental(ctx context.Context, oldSummary string, incremental []*schema.Message, forced bool, forcedPreserveTurns int) (newSummary string, consolidatedCount int, isRaw bool) {
	if len(incremental) <= 1 {
		return oldSummary, 0, false
	}

	// 1. 拆当前轮：最后一条 user 及其后原样保留（本次对话直接用）
	lastUserIdx := -1
	for i := len(incremental) - 1; i >= 0; i-- {
		if incremental[i].Role == schema.User {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx <= 0 {
		return oldSummary, 0, false
	}

	history := incremental[:lastUserIdx]
	tail := incremental[lastUserIdx:]

	// 2. 保留策略：把旧摘要折算为一条 system 消息参与预算计算，
	//    使增量模式与全量模式的保留策略一致（复用 findKeepBoundary 的 tool 配对保护）；
	//    再叠加保底保留最近 N 轮原文（自动用 c.preserveTurns，强制用调用方指定）
	systemMsgs := []*schema.Message{}
	if oldSummary != "" {
		systemMsgs = append(systemMsgs, &schema.Message{Role: schema.System, Content: oldSummary})
	}
	keepFromEnd := c.findKeepBoundary(history, int(float64(c.maxTokens)*c.threshold*consolidationTargetRatio), systemMsgs, c.estimator.EstimateMessages(tail))
	if forced {
		// 强制模式：以保底保留为上限（尽量多压）；保底不可用（超窗）时保持预算结果防爆窗
		if minKeep := c.preserveTurnsKeep(history, tail, forcedPreserveTurns); minKeep >= 0 && minKeep < keepFromEnd {
			keepFromEnd = minKeep
		}
	} else {
		keepFromEnd = c.applyPreserveTurns(keepFromEnd, history, tail)
	}

	// 3. 预算足够保留全部增量 → 无需压缩，摘要保持不变
	if keepFromEnd >= len(history) {
		return oldSummary, 0, false
	}

	toConsolidate := history[:len(history)-keepFromEnd]
	logger.Infof("[MemoryConsolidator] 增量压缩：%d 条新消息并入摘要（保留 %d 条）",
		len(toConsolidate), keepFromEnd)

	// 4. LLM 合并旧摘要与增量，失败后降级为"旧摘要 + 原文归档"
	newSummary, err := c.mergeSummaryWithRetry(ctx, oldSummary, toConsolidate)
	if err != nil {
		logger.Warnf("[MemoryConsolidator] 增量摘要合并失败，降级为原文归档: %v", err)
		if oldSummary != "" {
			newSummary = oldSummary + "\n\n[Raw archive of newly added messages (LLM summarization unavailable)]\n\n" + rawArchive(toConsolidate)
		} else {
			newSummary = c.rawArchiveSummary(toConsolidate)
		}
		return newSummary, len(toConsolidate), true
	}
	return newSummary, len(toConsolidate), false
}

// mergeSummaryWithRetry 调用 LLM 将旧摘要与增量消息合并为新摘要，失败重试
func (c *Consolidator) mergeSummaryWithRetry(ctx context.Context, oldSummary string, incremental []*schema.Message) (string, error) {
	m, err := c.getModel(ctx)
	if err != nil {
		return "", fmt.Errorf("创建摘要模型失败: %w", err)
	}

	prompt := c.buildIncrementalPrompt(oldSummary, incremental)
	var lastErr error
	for attempt := 1; attempt <= maxConsolidationAttempts; attempt++ {
		summarizeCtx, cancel := context.WithTimeout(ctx, consolidationTimeout)
		resp, err := m.Generate(summarizeCtx, []*schema.Message{
			{Role: schema.System, Content: consolidationSystemPrompt},
			{Role: schema.User, Content: prompt},
		}, model.WithTemperature(consolidationTemperature), model.WithMaxTokens(consolidationPromptMaxTokens))
		cancel()

		if err != nil {
			lastErr = err
			logger.Warnf("[MemoryConsolidator] 增量摘要合并尝试 %d/%d 失败: %v", attempt, maxConsolidationAttempts, err)
			continue
		}
		if resp != nil && resp.Content != "" {
			return resp.Content, nil
		}
		lastErr = fmt.Errorf("LLM 返回空摘要")
	}
	return "", fmt.Errorf("增量摘要合并失败 %d 次: %w", maxConsolidationAttempts, lastErr)
}

// buildIncrementalPrompt 构造增量合并 prompt：旧摘要 + 新增对话 → 合并后的完整摘要
func (c *Consolidator) buildIncrementalPrompt(oldSummary string, incremental []*schema.Message) string {
	var sb strings.Builder
	sb.WriteString("You are merging new conversation messages into an existing summary.\n")
	sb.WriteString("The new summary MUST be the complete, self-contained summary that includes BOTH the existing summary content and the new information.\n\n")

	sb.WriteString("Existing summary of earlier conversation:\n")
	sb.WriteString("---\n")
	if oldSummary == "" {
		sb.WriteString("(none)\n")
	} else {
		sb.WriteString(truncateString(oldSummary, promptUserMaxChars*2))
	}
	sb.WriteString("\n---\n\n")

	sb.WriteString("New conversation messages to merge:\n\n")
	for _, msg := range incremental {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.User:
			fmt.Fprintf(&sb, "**User**: %s\n\n", truncateString(msg.Content, promptUserMaxChars))
		case schema.Assistant:
			if len(msg.ToolCalls) > 0 {
				names := make([]string, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					names = append(names, tc.Function.Name)
				}
				fmt.Fprintf(&sb, "**Assistant** [called tools: %s]: %s\n\n",
					strings.Join(names, ", "), truncateString(msg.Content, promptToolCallMaxChars))
			} else {
				fmt.Fprintf(&sb, "**Assistant**: %s\n\n", truncateString(msg.Content, promptAssistantMaxChars))
			}
		case schema.Tool:
			fmt.Fprintf(&sb, "**Tool [%s]**: %s\n\n", msg.ToolName, truncateString(msg.Content, promptToolMaxChars))
		}
	}
	return sb.String()
}

// preserveTurnsKeep 计算"保底保留最近 turns 轮原文"的消息条数：
// 历史不足 turns 轮返回 0（无可保底）；窗口校验不通过（保底轮数原文 + 当前轮 +
// 摘要预留超过 maxTokens）返回 -1（表示保底不可用，调用方退化为预算驱动）。
func (c *Consolidator) preserveTurnsKeep(history []*schema.Message, tail []*schema.Message, turns int) int {
	if turns <= 0 {
		return -1
	}
	keepStart := tailKeepStart(history, turns)
	if keepStart == 0 {
		return 0
	}
	minKeep := len(history) - keepStart
	// 窗口校验：保底轮数原文 + 当前轮 + 摘要预留 ≤ maxTokens 才允许保底
	keepTokens := c.estimator.EstimateMessages(history[keepStart:])
	tailTokens := c.estimator.EstimateMessages(tail)
	if keepTokens+tailTokens+summaryReserveTokens <= c.maxTokens {
		return minKeep
	}
	return -1
}

// applyPreserveTurns 在预算保留的基础上叠加"保底保留最近 N 轮原文"：
// 用户显式配置的轮数（如 3）保证最近 3 轮对话始终以原文形式保留，
// 只有更早的历史才被压缩。校验规则：保底轮数 + 当前轮 + 摘要预留
// 不超过模型窗口（maxTokens）时才生效，否则退化为预算驱动，保证不撑爆窗口。
func (c *Consolidator) applyPreserveTurns(keepFromEnd int, history []*schema.Message, tail []*schema.Message) int {
	if c.preserveTurns <= 0 {
		return keepFromEnd
	}
	minKeep := c.preserveTurnsKeep(history, tail, c.preserveTurns)
	if minKeep <= keepFromEnd {
		return keepFromEnd
	}
	return minKeep
}

// findKeepBoundary 从历史尾部回扫，计算在预算内最多保留多少条消息。
// 遇到 tool 消息时，将连续 tool 组与其前驱 assistant 整体计算——
// 要么整组保留，要么整组压缩，保证 tool_call 与 tool result 配对完整。
func (c *Consolidator) findKeepBoundary(history []*schema.Message, targetTokens int, systemMsgs []*schema.Message, tailTokens int) int {
	budget := targetTokens -
		c.estimator.EstimateMessages(systemMsgs) -
		tailTokens -
		summaryReserveTokens
	if budget <= 0 {
		return 0
	}

	tokens := 0
	keepCount := 0
	i := len(history) - 1
	for i >= 0 {
		msg := history[i]
		msgTokens := c.estimator.EstimateMessage(msg)

		if msg.Role == schema.Tool {
			// 工具结果组：连续 tool 消息 + 触发它们的前驱 assistant 整体保留
			groupTokens := msgTokens
			groupSize := 1
			j := i - 1
			for j >= 0 && history[j].Role == schema.Tool {
				groupTokens += c.estimator.EstimateMessage(history[j])
				groupSize++
				j--
			}
			if j >= 0 && history[j].Role == schema.Assistant {
				groupTokens += c.estimator.EstimateMessage(history[j])
				groupSize++
			}

			if tokens+groupTokens > budget {
				break
			}
			tokens += groupTokens
			keepCount += groupSize
			i -= groupSize
		} else {
			if tokens+msgTokens > budget {
				break
			}
			tokens += msgTokens
			keepCount++
			i--
		}
	}
	return keepCount
}

// summarizeWithRetry 调用 LLM 生成摘要，失败重试 maxConsolidationAttempts 次
func (c *Consolidator) summarizeWithRetry(ctx context.Context, messages []*schema.Message) (string, error) {
	m, err := c.getModel(ctx)
	if err != nil {
		return "", fmt.Errorf("创建摘要模型失败: %w", err)
	}

	prompt := c.buildConsolidationPrompt(messages)
	var lastErr error
	for attempt := 1; attempt <= maxConsolidationAttempts; attempt++ {
		summarizeCtx, cancel := context.WithTimeout(ctx, consolidationTimeout)
		resp, err := m.Generate(summarizeCtx, []*schema.Message{
			{Role: schema.System, Content: consolidationSystemPrompt},
			{Role: schema.User, Content: prompt},
		}, model.WithTemperature(consolidationTemperature), model.WithMaxTokens(consolidationPromptMaxTokens))
		cancel()

		if err != nil {
			lastErr = err
			logger.Warnf("[MemoryConsolidator] 摘要尝试 %d/%d 失败: %v", attempt, maxConsolidationAttempts, err)
			continue
		}
		if resp != nil && resp.Content != "" {
			return resp.Content, nil
		}
		lastErr = fmt.Errorf("LLM 返回空摘要")
	}
	return "", fmt.Errorf("摘要生成失败 %d 次: %w", maxConsolidationAttempts, lastErr)
}

// getModel 懒创建摘要模型并缓存（互斥保护，避免并发重复创建）
func (c *Consolidator) getModel(ctx context.Context) (model.BaseModel[*schema.Message], error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.model == nil && c.modelErr == nil {
		c.model, c.modelErr = c.createModel(ctx)
	}
	return c.model, c.modelErr
}

// buildConsolidationPrompt 将待压缩消息转为摘要 prompt 文本
func (c *Consolidator) buildConsolidationPrompt(messages []*schema.Message) string {
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation history, preserving:\n")
	sb.WriteString("1. Key facts and decisions made\n")
	sb.WriteString("2. Tool execution results and their outcomes\n")
	sb.WriteString("3. User's original intent and requirements\n")
	sb.WriteString("4. Any errors encountered and how they were resolved\n\n")
	sb.WriteString("Conversation to summarize:\n\n")

	for _, msg := range messages {
		if msg == nil {
			continue
		}
		switch msg.Role {
		case schema.User:
			fmt.Fprintf(&sb, "**User**: %s\n\n", truncateString(msg.Content, promptUserMaxChars))
		case schema.Assistant:
			if len(msg.ToolCalls) > 0 {
				names := make([]string, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					names = append(names, tc.Function.Name)
				}
				fmt.Fprintf(&sb, "**Assistant** [called tools: %s]: %s\n\n",
					strings.Join(names, ", "), truncateString(msg.Content, promptToolCallMaxChars))
			} else {
				fmt.Fprintf(&sb, "**Assistant**: %s\n\n", truncateString(msg.Content, promptAssistantMaxChars))
			}
		case schema.Tool:
			fmt.Fprintf(&sb, "**Tool [%s]**: %s\n\n", msg.ToolName, truncateString(msg.Content, promptToolMaxChars))
		}
	}
	return sb.String()
}

// rawArchiveSummary 降级兜底：将待压缩消息转为纯文本归档（复用 rawArchive）
func (c *Consolidator) rawArchiveSummary(messages []*schema.Message) string {
	return "Raw conversation archive (LLM summarization unavailable):\n\n" + rawArchive(messages)
}

//nolint:lll // 摘要 system prompt 为完整语义文本
const consolidationSystemPrompt = "" +
	"你是一个对话摘要助手。你的任务是对用户与 AI 助手之间的对话生成简洁而全面的摘要。\n\n" +
	"摘要要求：\n" +
	"- 使用与原文相同的语言\n" +
	"- 保留所有关键事实、数字和具体细节\n" +
	"- 包含工具执行的结果和影响\n" +
	"- 记录遇到的错误及解决方式\n" +
	"- 涉及多个主题时用清晰的小节组织\n" +
	"- 尽量简洁——目标为原文长度的 30% 或更少\n\n" +
	"只输出摘要本身，不要任何前言或解释。"
