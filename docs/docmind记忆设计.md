# DocMind 对话上下文记忆设计

> 本文档说明 DocMind Agent 对话的**人机交互链路**与**多层级上下文记忆架构**，
> 帮助理解"为什么 Agent 能记住多轮对话、如何跨轮保持关键信息"。

## 一、总览

DocMind 的对话记忆采用**四层叠加**架构：

| 层级 | 名称 | 存储 | 作用域 | 核心机制 |
|------|------|------|--------|----------|
| 第 1 层 | 会话历史 | `messages` 表（PostgreSQL） | 单会话多轮 | 最近 N 轮消息回灌 |
| 第 2 层 | 短期记忆摘要 | `session_summaries` 表 | 单会话长上下文 | LLM 增量压缩 + 原文降级归档 |
| 第 3 层 | 长期记忆 | Neo4j 知识图谱 | 跨会话 | 对话信息提取落图 + 相关片段检索注入 |
| 第 4 层 | Agent 单轮状态 | ADK 内存态 | 单轮多次工具调用 | tool_call / tool_result 事件维护 |

**核心认知**：Agent 的"记忆"本质 = **落库的消息文本就是它的外部记忆**。
重要的中间产物（jobId、URL、pollUrl 等）必须写入回复文本，才能跨轮存活。

## 二、人机交互链路（一次对话如何跑通）

```
用户输入 "根据mysql的覆盖索引知识做成课件"
   ↓
前端 → POST /api/v1/agent-chat/{session_id}（SSE 流式请求）
   ↓
Gin controller → chatService.AgentChat()
   ↓
① 保存用户消息到 messages 表
② 加载 Agent 配置（内置模板 + 用户覆盖 override）
③ Registry 构建工具集（kb_search / MCP / python_exec / skill）
④ 加载历史上下文（多轮记忆，见第 3 节）
⑤ ADK 引擎运行：模型思考 → 调工具 → 生成回答
   ↓
事件流：state(thinking) → agent_step(tool_call/result) → answer(流式文本) → complete
   ↓
controller 映射 SSE 事件 → 前端逐块渲染
   ↓
完整回答落库 messages 表（role=assistant + agent_steps）
```

**传输层要点**：
- 回复通过 **SSE 事件流**输出（`answer` 事件逐块推送，前端打字机效果）
- 完整回复落库后，**刷新页面 / 重开历史会话仍可见**
- 下一轮对话加载历史时，Agent 能看到自己上轮的回复（多轮引用的基础）

## 三、四层记忆架构详解

### 第 1 层：会话历史（基础多轮记忆）

- **作用**：Agent 记住当前会话前几轮说了什么
- **机制**：每轮 user/assistant 消息落库 `messages` 表；下一轮加载最近 N 轮
  （`HistoryTurns`，内置默认 3 轮 → 最多 7 条消息）
- **代码位置**：`internal/service/chat_service.go` AgentChat 第 6 步
  `messageRepo.ListBySession(sessionID, historyTurns*2+1, nil)`
  → `historyToSchemaMessages()` 转换后拼入模型消息
- **注意**：
  - 只保留最近 N 轮，**超过的部分不再进入模型上下文**
  - `historyToSchemaMessages` 会跳过 content 为空的消息（工具调用轮次的
    assistant 消息无文本内容，空 content 会被部分模型服务端拒绝 400）

### 第 2 层：短期记忆摘要（长会话压缩）

- **作用**：会话很长时（历史超 Token 阈值），把早期内容压缩成摘要，
  只保留"摘要 + 最近增量"，避免上下文爆窗
- **机制**：
  1. 增量边界：`session_summaries` 表记录 `last_message_id`（已压缩边界）
  2. 边界之后的消息作为增量加载
  3. 摘要 + 增量超过阈值 → `Consolidator.ShouldConsolidate` 触发
  4. LLM 增量压缩新摘要并写回；失败降级为原文归档（raw 摘要）
- **代码位置**：
  - `internal/memory/consolidator.go`（LLM 增量压缩）
  - `internal/memory/degrade.go`（原文降级归档）
  - `internal/service/chat_service.go` KnowledgeChat 第 4.1 步（手动压缩）
- **应用范围**：当前仅 knowledge-chat（快速问答）接入；agent-chat 未接摘要压缩

### 第 3 层：长期记忆（跨会话知识图谱）

- **作用**：跨会话记住用户偏好、历史事实（如"用户之前问过 MySQL 优化"）
- **机制**：
  1. 每次对话结束（或按策略）由 LLM 提取实体/关系
  2. 异步写入 Neo4j 知识图谱（`AddEpisode`）
  3. 新对话开始时 `RetrieveMemory` 检索相关历史片段，注入当前用户问题
     （只注入发给模型的 user 消息，不改原始 query）
- **代码位置**：
  - `internal/memory/longterm/extractor.go`（对话信息提取器）
  - `internal/memory/longterm/service.go`（AddEpisode 异步落图）
  - `internal/service/chat_service.go` AgentChat 第 6.5 步（检索注入）
- **失败兜底**：检索失败仅记日志跳过注入，不阻断对话

### 第 4 层：Agent 单轮内部状态（ADK）

- **作用**：一轮对话内多次工具调用的中间结果串联
  （如：skill 加载 → kb_search 检索 → make_request 提交 → 查询进度）
- **机制**：eino ADK 引擎维护消息状态机，每次工具调用的
  `tool_call / tool_result` 作为消息回灌给模型，直到模型认为可作答
- **落库**：轮次结束后压缩进 `messages.agent_steps`（JSON 数组），
  供前端展示步骤记录、后续排查

## 四、两种对话模式记忆差异

| 模式 | 第 1 层历史 | 第 2 层摘要 | 第 3 层长期记忆 | 第 4 层工具状态 |
|------|------------|------------|----------------|-----------------|
| **agent-chat**（智能推理） | ✅ 最近 N 轮 | ❌ 未接入 | ✅ 注入 | ✅ |
| **knowledge-chat**（快速问答） | ✅ 增量消息 | ✅ 增量压缩 | ✅ 注入 | ✅ |

## 五、实践要点：关键信息如何跨轮存活

**问题**：Agent 的历史记忆只保留最近 N 轮，且"记忆"= 消息文本。
若中间产物（jobId、pollUrl、URL）没有写进回复文本，下一轮 Agent 就会丢失。

**规则**：
1. **重要中间产物必须写入回复文本**（如"Job ID: xxx"），落库后成为
   下一轮可引用的历史
2. 需要跨轮使用的查询路径/参数，在技能文档中**显式固化**
   （如 openmaic-classroom 技能的 Phase 4：
   `GET {base_url}/api/generate-classroom/{jobId}` 是唯一查询端点，
   禁止猜测 `/api/jobs` 等路径）
3. 异步任务（课件生成等）提交后：
   - 回复中写明 jobId + 查询方式 + 预计时长
   - 用户追问时用**同一路径**查询，succeeded 后回复最终 URL

**案例复盘**（openmaic-classroom 真实故障）：
```
21:25 提交成功 → 回复"提交成功"（未写 jobId/pollUrl）
21:34 用户追问 → Agent 历史里没有 jobId → 瞎猜路径 → 全部 404
→ 课件其实早已生成完成，用户却拿不到链接
→ 修复：SKILL.md 强制要求提交后把 jobId/pollUrl 写入回复 + 固定查询端点
```

## 六、常见问题

**Q1：为什么 Agent 会"忘记"之前说的内容？**
- 会话历史只保留最近 N 轮（第 1 层），超出的被截断
- 中间产物没写进回复文本，落库后无从引用
- 跨会话内容需第 3 层长期记忆（Neo4j）支持

**Q2：上下文会不会爆窗？**
- 第 1 层按轮数截断；第 2 层按 Token 阈值触发摘要压缩
- 模型上下文窗口有缺失回填机制（`model_context_window_missing`）

**Q3：历史里为什么会有空消息？**
- 工具调用轮次的 assistant 消息 content 可能为空（正常现象）
- 发送前统一过滤，避免模型服务端拒绝（`missing field content` 400）

## 七、相关代码索引

| 关注点 | 文件 |
|--------|------|
| Agent 历史加载（第 1 层） | `internal/service/chat_service.go` AgentChat |
| 摘要压缩（第 2 层） | `internal/memory/consolidator.go`、`degrade.go`、`summary_middleware.go` |
| 长期记忆（第 3 层） | `internal/memory/longterm/`（extractor / service / neo4j_repository） |
| 引擎事件流（第 4 层） | `internal/agent/engine.go`、`runner.go`、`state.go` |
| 技能记忆规范 | `configs/skills/openmaic-classroom/SKILL.md` Phase 4 |
