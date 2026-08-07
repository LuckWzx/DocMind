# Agent 模式增量记忆接入说明（交付乙）

> 文档状态：2026-08-07 由甲交付
> 对应代码：[internal/memory/incremental_context.go](../internal/memory/incremental_context.go)、[internal/memory/consolidator.go](../internal/memory/consolidator.go)
> 配套表：`session_summaries`（GORM AutoMigrate 自动创建，无需手动建表）

---

## 一、背景：为什么需要这次接入？

你已经接入了 `NewSummaryMiddleware`（官方 summarization 中间件），它解决的是**单次 Agent 运行内**的上下文超限问题：

```
一次运行内（状态常驻）：推理 → 工具 → 再推理，摘要压缩后留在 state 里
→ 第二次压缩的输入天然是"旧摘要 + 新消息" ✅ 增量
```

但它**不解决跨请求**的问题：

```
每次请求：调用方从数据库重新组装 req.Messages → runner.Run 基于新消息建 state
→ 运行结束 state 丢弃 → 摘要没有落库 → 下次请求全量重算 ❌
```

> 根因：官方中间件在 eino 内部运行，拿不到 `sessionID`，也拿不到消息的数据库 ID（`schema.Message` 没有 ID 字段），无法自己读写摘要表。

**本方案**：在组装 `RunRequest.Messages` 的地方调用一个封装函数 `BuildAgentContext`，一次性完成"读摘要 → 增量加载 → 触发压缩 → 写回 → 拼装"，实现跨请求增量压缩。官方中间件保留，作为单次运行内（长工具链）的兜底。

## 二、改动量：一行

组装 Messages 处（现在）：

```go
req := &agent.RunRequest{
    Messages: append(loadHistory(sessionID), currentMsg), // 现在的全量加载
    SessionID: sessionID,
    UserID:    userID,
    Agent:     agt,
}
```

改为（接入后）：

```go
consolidator := memory.NewConsolidator(
    func(ctx context.Context) (model.BaseModel[*schema.Message], error) {
        return chatModelFactory.CreateChatModel(ctx, agentCfg.ModelID)
    },
    tokenEstimator,          // token.NewEstimator()，可复用 chat_service 的
    memory.DefaultMaxContextTokens,
    0,                       // 触发比例默认 0.5
)

req := &agent.RunRequest{
    Messages: memory.BuildAgentContext(
        ctx, sessionID,
        summaryRepo,     // repository.NewSummaryRepository(db)
        messageRepo,     // repository.NewMessageRepository(db)
        consolidator,
        historyTurns,    // 首次加载的历史轮数（每轮 2 条），无摘要时生效
        currentMsg,      // 当前轮用户消息（两种传法见下文）
    ),
    SessionID: sessionID,
    UserID:    userID,
    Agent:     agt,
}
```

> `BuildAgentContext` 返回完整消息列表（`[摘要 system 消息（若有）] + 增量保留部分 + 当前轮`），**直接作为 `req.Messages` 传给引擎即可**。加载/压缩失败会返回 error，调用方记日志后回退为"只传当前消息"，不阻断对话。

## 三、函数契约

```go
func BuildAgentContext(
    ctx context.Context,
    sessionID uint,
    summaryStore SummaryStore,     // repository.SummaryRepository 可直接传入（接口自动满足）
    messageLoader MessageLoader,   // repository.MessageRepository 可直接传入
    consolidator *Consolidator,    // memory.NewConsolidator(...)
    historyTurns int,              // ⚠️ 已废弃：历史加载量由压缩机制接管，传任意值均不影响行为（兼容旧调用方）
    currentUserMsg *schema.Message,// 当前轮用户消息（见下方两种用法）
) ([]*schema.Message, error)
```

> **2026-08-07 更新**：`historyTurns` 参数已废弃。原实现"无摘要时只加载最近 N 轮"会导致 Token 永远达不到压缩阈值、摘要永不产生（滑动窗口掐死压缩触发）。现改为无摘要时**全量加载**（上限 500 条），Token 自然累积触发首份摘要后进入增量模式。乙侧无需改动调用代码。

### 内部流程（你不需要关心，但要知道语义）

```
① GetBySession 读旧摘要
② 有摘要 → ListAfterID(LastMessageID) 只加载边界后的增量消息
   无摘要 → ListBySession(historyTurns*2+1) 加载最近 N 轮
③ 估算 Token（摘要 + 增量）→ 超过阈值（默认 50% × 160k）触发
④ ConsolidateIncremental：LLM 把"旧摘要 + 增量"合并成新摘要
   → Upsert 写回 session_summaries（LastMessageID = 被压缩最后一条消息 ID）
   → 本次请求用新摘要 + 保留的增量
⑤ 未超阈值 → 摘要不变，原样拼装
```

### 两种调用方式（二选一，推荐 ①）

| 方式 | 做法 | currentUserMsg 传参 |
|---|---|---|
| ① 先存库（推荐，与 quick-answer 一致） | 先 `messageRepo.Create` 保存当前问题，再调用 | 传 `nil`（函数把加载结果整体当增量，最后一条 user 即当前轮） |
| ② 后传参 | 当前问题不存库 | 传当前问题消息（函数追加为增量最后一条） |

## 四、语义保证

| 场景 | 行为 |
|---|---|
| 触发压缩 | 摘要写回 `session_summaries`，边界前进；本次请求立即用新摘要 |
| 未触发 | 摘要表不变，旧摘要 + 增量原样拼装 |
| LLM 摘要失败（3 次重试后） | 降级为**保留旧摘要 + 增量原文归档**（`summary_type='raw'`），不阻断对话 |
| 首次（无摘要） | 等价于全量压缩一次，之后进入增量模式 |
| 删除/清空会话 | 需联动调 `summaryRepo.DeleteBySession(sessionID)`（chat_service 已做，Agent 侧如有删除入口请同步） |
| 并发 | 边界只会前进；并发写冲突最多导致增量重复合并，消息永不丢失 |

## 五、官方中间件与 BuildAgentContext 的分工

```
┌─ 跨请求（每次对话）──────────────────────────────┐
│  BuildAgentContext：读摘要 → 增量加载 → 压缩 → 写回 │  ← 本次新增
└──────────────────────────────────────────────────┘
                      ↓ req.Messages
┌─ 单次运行内（多轮 LLM 调用）──────────────────────┐
│  官方 summarization 中间件：运行内超限再压缩（兜底）  │  ← 已接入
└──────────────────────────────────────────────────┘
```

两者互不干扰：`BuildAgentContext` 处理跨请求的长期膨胀，官方中间件处理单次运行内（长工具链）的临时膨胀。

## 六、验证方式

- 单测（甲已交付）：`go test ./internal/memory/...`，覆盖 6 个场景（有摘要超限 / 增量不足 / 首次压缩 / 降级 / 参数校验 / 加载失败）
- 联调建议（100 轮端到端）：
  1. 组装 Messages 前打日志：`summary=有/无, incremental=N 条, tokens=X`
  2. 触发压缩后查库：`SELECT * FROM session_summaries WHERE session_id=?`，确认 `last_message_id` 前进、`content` 更新
  3. 第 N+1 轮请求确认日志显示 `incremental` 只含边界后的消息（而不是全部历史）
  4. 拔掉摘要模型（或故意配错）确认对话不中断、`summary_type='raw'`

## 七、相关代码位置

| 文件 | 说明 |
|---|---|
| [internal/memory/incremental_context.go](../internal/memory/incremental_context.go) | `BuildAgentContext` + 本地接口定义（SummaryStore / MessageLoader） |
| [internal/memory/consolidator.go](../internal/memory/consolidator.go) | `ConsolidateIncremental` 增量合并核心 |
| [internal/model/entity/session_summary.go](../internal/model/entity/session_summary.go) | `session_summaries` 表实体 |
| [internal/repository/summary_repository.go](../internal/repository/summary_repository.go) | 摘要读写仓储 |
| [internal/repository/message_repository.go](../internal/repository/message_repository.go) | `ListAfterID` 增量加载 |
| [internal/service/chat_service.go](../internal/service/chat_service.go) | quick-answer 模式同构实现（参考） |
