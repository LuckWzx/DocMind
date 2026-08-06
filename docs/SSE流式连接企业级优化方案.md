# SSE 流式连接企业级优化方案

> 状态：规划中（未实施）
> 创建时间：2026-08-06
> 背景：当前 SSE 采用"请求级短连接"（每问一答建一次连接），大方向正确，但缺少企业级能力（事件协议、心跳、幂等、流凭证、可观测、水平扩展、优雅关闭）。

---

## 一、核心结论

**"请求级 SSE 短连接"（每问一答建一次连接）本身就是正确的大方向**，与 OpenAI、Anthropic 等主流 AI 产品一致，不需要改成"登录时建立常驻连接"。但当前实现距离企业级还差 6 个关键能力：

| 维度 | 现状 | 企业级差距 |
|---|---|---|
| 事件协议 | 全部 `event: message`，无事件 ID | 无法断点续传、无法区分心跳 |
| 可靠性 | 无心跳、无幂等 | 代理断连、重复提交双扣费 |
| 安全 | JWT 全程生效 | 连接期内 token 失效风险、无限流 |
| 可观测 | 有 TTFB 日志 | 无连接级指标、无生命周期日志 |
| 弹性 | 连接绑定实例 | 多实例部署会断流 |
| 生命周期 | 无优雅关闭 | 发版时用户看到"断流" |

---

## 二、现状梳理（改造前基线）

### 后端
- 路由：`POST /api/v1/chat/knowledge-chat/:session_id`（`internal/api/v1/chat/routes.go`）
- Handler：`KnowledgeChat`（`internal/api/v1/chat/controller.go`）——设置 SSE 头 → 调 `chatService.KnowledgeChat()` 拿 StreamReader → 循环 Recv 推送 `references`/`answer` 事件 → 保存消息 → 推 `session_title`/`complete`
- 事件格式：`writeSSEEvent` 固定 `event: message` + JSON（`response_type` 区分类型）
- 鉴权：`middleware.Auth()` JWT Bearer，24h 过期（`pkg/jwt`）
- 断连检测：handler 使用 `c.Request.Context()`，客户端断开会自动取消 LLM 流（已正确，保持）

### 前端
- `web/src/api/chat/streame.ts`：`@microsoft/fetch-event-source` 的 `fetchEventSource`
- 已有能力：AbortController、`streamGeneration` 防串流、缓冲渲染节流、TTFB 双端日志、`X-Request-ID`（12 位随机串）
- 事件消费：`web/src/composables/useChatStreamHandler.ts` 按 `response_type` 分发

### 基础设施
- nginx 已配 SSE 相关：`proxy_http_version 1.1`、`proxy_buffering off`、`proxy_read_timeout 3600s`、`X-Accel-Buffering no`（后端已设置）
- 已有 Redis（`configs/config.yaml`），可复用做幂等去重、流凭证、事件暂存
- embed 场景已有独立 Embed Token + session 签名机制

### 关联演进规划
- 阶段一（当前）：compose.Graph 确定性 RAG 流水线
- 阶段二（后续）：ADK 架构（ChatModelAgent + Runner），多轮事件流（agent_query/thinking/tool_call 等）——**事件协议需现在预留扩展**

---

## 三、方案设计

### 1. 协议层：事件契约规范化（第一批）

当前所有事件写死 `event: message`。改造为四类事件 + 事件 ID：

```
event: message    data: {"id":"req_xxx:3","response_type":"answer",...}   # 业务数据
event: ping       data: {"timestamp": ...}                                  # 心跳，每15-30s
event: error      data: {"code":"LLM_TIMEOUT","retryable":true,"retry_after_ms":3000}
event: done       data: {"task_id":...}                                     # 明确结束信号
```

要点：
- **每个业务事件带自增 `id`**（`requestID:seq`）：前端断线后带 `Last-Event-ID` 重连，后端从 Redis buffer（保留 30~60s）重放未消费片段
- **错误事件契约化**：`error_code` + `retryable` + `retry_after_ms`，前端据此决定自动重试或提示用户（替代现在的直接 throw）
- **心跳**：每 15~30s 发送，规避云 LB / NAT 空闲断连；`event: ping` 或 `: ping` 注释行均可
- **协议为阶段二 ADK 预留**：定义事件枚举 + payload 结构（thinking/tool_call/agent_query 等），避免阶段二返工

### 2. 安全：短期流凭证 + 限流（第一批）

- **二段式流凭证 `stream_ticket`**：POST 提交（JWT 鉴权）→ 返回一次性 ticket（Redis 存储，TTL 5min，绑定 `user_id + session_id`，单次消费）；SSE 连接用 ticket 建立，建立后即销毁。同时统一 embed 场景的凭证体系
- **限流**：SSE 端点独立 per-user QPS 限流（Redis 令牌桶）
- **执行护栏**：总执行超时（如 5min）、首 token 超时（如 30s 无响应发 error）、请求体大小限制

### 3. 可靠性：幂等 + 优雅关闭（第一批）

- **幂等去重**：前端已带 `X-Request-ID`，后端用 `Redis SETNX key=req_id NX EX 30` 去重，防止超时重试导致 LLM 双倍调用（双倍费用、双条消息）
- **优雅关闭**：SIGTERM 时对活跃 SSE 发 `error` 事件（code=`SERVER_SHUTDOWN`, retryable=true）+ 等待 5s 再退出，前端自动重连
- 断连检测沿用现有 `c.Request.Context()` 机制

### 4. 可观测性：SSE 生命周期指标（第一批）

- **指标**（Prometheus 或日志聚合）：活跃连接数、事件推送数/秒、首 token 延迟 P50/P99、连接时长、完成率、错误分类
- **生命周期日志**：`sse_open / sse_event / sse_close(abort|done|error)` 四个日志点，带 request_id 贯穿（现有 TTFB 日志基础上扩展）
- **全链路追踪**：后续上 OpenTelemetry 时，把 LLM 调用 → 向量检索 → SSE 推送串成一条 trace

### 5. 水平扩展：提交/消费分离（第二批，多实例时再做）

当前架构短板：SSE 连接与后端实例绑定，多实例 + LB 时 `proxy_next_upstream` 重试会重复执行 LLM、连接漂移断流。

推荐演进为**任务队列模式**（复用现有 Redis）：

```
POST /chat → 鉴权 + 入 Redis Stream（事件暂存）+ 返回 task_id   ← 任意实例可处理
GET  /events/:task_id → 任意实例提供 SSE，订阅 Redis Pub/Sub  ← 连接可漂移、可重连
```

收益：后端无状态、天然水平扩展、断线重连从"重放 buffer"升级为"重放 Stream"、滚动升级不影响在途对话。代价：TTFB 略增（毫秒级）、引入消息中间件依赖。

**注意**：不要用 LB sticky session 替代，那只解决路由，不解决重试双执行问题。

### 6. 前端配套：StreamClient 抽象（第三批）

`streame.ts` 中流逻辑与 UI 耦合。抽取独立 `StreamClient`：

- 能力：心跳监听、指数退避自动重连、`retryable` 错误自动重试、网络 offline 感知
- 重连恢复策略：优先 `Last-Event-ID` 续传，不可续传降级为重新拉取消息列表（`getmsgList` 兜底）
- UI 层只消费事件，不感知连接细节
- 保留现有已正确的机制：AbortController、streamGeneration 防串流、缓冲渲染节流

---

## 四、落地路线

| 批次 | 内容 | 工作量 | 触发条件 |
|---|---|---|---|
| **第一批** | 事件协议规范化（id/心跳/error 契约）、幂等去重、执行护栏、SSE 指标与生命周期日志、优雅关闭 | 1-2 天 | 建议立即做 |
| **第二批** | stream_ticket 流凭证、限流、提交/消费分离（Redis Stream + Pub/Sub） | 3-5 天 | 多实例部署时做 |
| **第三批** | 前端 StreamClient 重构、断线续传、网络感知 | 1-2 天 | 与第二批联动 |

第一批改动集中在：新建 `internal/api/sse.go`（事件协议工具包）+ `internal/api/v1/chat/controller.go` 微调 + 新增幂等/限流中间件，风险低、收益立竿见影。

第二、三批建议与阶段二 ADK 上线一起规划（ADK 多轮事件流天然需要更可靠的事件通道）。

---

## 五、涉及文件清单

### 后端
- `internal/api/v1/chat/controller.go` — SSE handler 改造（事件 ID、心跳、错误契约、优雅关闭钩子）
- `internal/api/v1/chat/routes.go` — 路由（第二批：拆分为 POST 提交 + GET 订阅）
- `internal/middleware/` — 新增幂等去重、限流中间件（第一批）；流凭证校验（第二批）
- 新增：`internal/api/sse.go`（或 `pkg/sse/`）— 事件协议工具包（Event 类型、ID 生成、心跳、Writer 封装）
- 新增：SSE 指标与生命周期日志（复用 `pkg/logger`）
- `internal/service/chat_service.go` — 执行护栏（总超时、首 token 超时）
- `configs/config.yaml` — 新增 SSE 配置节（心跳间隔、超时、限流、buffer TTL）

### 前端
- `web/src/api/chat/streame.ts` — 重构为 StreamClient（第三批）
- `web/src/composables/useChatStreamHandler.ts` — 事件分发增加 error 契约处理、重连恢复
- `web/src/views/chat/index.vue` — 网络状态感知、重连提示

### 基础设施
- `web/nginx.conf` — 已具备 SSE 代理配置，无需大改（心跳上线后可适当缩短 `proxy_read_timeout`）
