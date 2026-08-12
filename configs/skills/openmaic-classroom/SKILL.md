---
name: openmaic-classroom
description: 将 RAG 检索结果或文档块转换为 OpenMAIC 互动课程。当用户要求将知识库内容、检索到的文档片段、或上传的文档转换为教学课件/互动课堂时使用此技能。支持纯需求生成和基于 PDF 内容的课程生成。
---

# OpenMAIC Classroom Generator

将 DocMind 知识库中的 RAG 检索结果或文档内容转换为 OpenMAIC 互动课程。

## 核心能力

1. **RAG → 课程**: 将知识检索结果提炼为教学需求（requirement），通过 OpenMAIC API 生成互动课程
2. **PDF → 课程**: 解析用户上传的 PDF，结合内容生成课程
3. **文档块 → 课程集**: 将多个文档块/知识片段组织为多阶段课程集

## 能力边界

> **通过 DocMind 注册的 mcp_api_requester MCP 工具，你可以直接调用 OpenMAIC API**（HTTP POST/GET 请求）。
> 该工具在 DocMind 中注册为 `mcp_{service_name}_{tool_name}` 格式（如 `mcp_mcp_api_requester_make_request`）。
> **必须**使用该 MCP 工具调用 API。如果 MCP 工具未配置或不可用，引导用户先部署 mcp-api-requester，不提供 curl 命令。

> **⚠️ MCP 可用性检查（每次读取此文件后必须执行）：**
> 在开始工作前，检查可用的 MCP 工具列表中是否存在名称包含 `mcp_api_requester` 的工具。
> - **如果未找到**：必须提醒用户：“未检测到 `mcp_api_requester` MCP 服务。请先从 https://github.com/yryuu/mcp-api-requester 下载代码并部署，然后在 DocMind 中注册该 MCP 服务。”
> - **如果已找到**：继续后续流程，使用该 MCP 工具调用 API。

## 模式选择

OpenMAIC 有两种使用模式，**根据用户场景选择**：

| 模式 | Base URL | 认证方式 | 适用场景 |
|------|----------|----------|----------|
| 托管模式（推荐快速使用） | `https://open.maic.chat` | `Authorization: Bearer <access-code>` | 用户有 open.maic.chat 访问码，无需本地部署 |
| 本地模式 | 用户提供（见本地模式 Base URL 处理） | 无认证（本地自部署） | 用户自行部署了 OpenMAIC 实例 |

**判断规则**：
- 用户提到"在线服务"、"open.maic.chat"、"访问码" → 使用托管模式
- 用户提到"本地部署"、"自建" → 使用本地模式
- 用户未明确说明时，优先询问用户使用哪个模式

**本地模式 Base URL 处理**：
1. 用户选择本地模式后，必须询问用户：“请输入你的 OpenMAIC 本地部署地址（例如 `http://localhost:3000` 或 `http://192.168.1.100:3000`）”
2. 收到用户提供的地址后直接使用，**无需任何替换**——DocMind 为本地进程，`localhost` / `127.0.0.1` 指向本机，可直接访问宿主机服务

## 前置条件

| 配置项 | 说明 |
|--------|------|
| 模式 | 托管模式 或 本地模式（见上方判断规则） |
| `accessCode` | 托管模式必需——访问码（以 `sk-` 开头），由用户在 open.maic.chat 获取 |
| 健康检查 | 调用前验证服务可用：`GET <BASE_URL>/api/health` |

## 使用场景

当用户请求涉及以下内容时，使用此技能：
- "把这个文档做成课件"
- "基于检索结果生成课程"
- "为这个知识点创建互动课堂"
- "将知识库内容转换为教学材料"

## 工作流程

### Phase 1: 确认输入源

确认课程生成的输入来源（三选一）：

1. **纯需求生成**: 用户直接描述教学主题，无需额外文档
   → 直接使用用户描述作为 `requirement`，**无需调用脚本**
2. **RAG 检索结果**: 先通过 `kb_search` 检索相关知识（或使用主对话已完成的检索结果），再将结果组织为 requirement
   → 按 `references/requirement-builder.md` 模板构建结构化 requirement（见 Phase 1.1）
3. **PDF 文件**: 用户提供 PDF 文件路径，先解析再调用生成 API
   → 提取 PDF 文本后构建 requirement，**无需调用脚本**

### Phase 1.1: RAG 结果 → Requirement 转换（仅适用于场景 2）

根据检索结果（chunks），按照 `references/requirement-builder.md` 的模板**直接构建结构化 requirement**：

1. 从检索结果中提取：核心主题/概念、关键知识点、文档来源信息
2. 按模板 2（基于 RAG 检索结果）组织 requirement 文本：

```
基于以下知识内容，创建一个面向[目标受众]的[深度级别]课程：

核心主题：[从检索结果提取的主要概念]
关键知识点：
- [知识点1]
- [知识点2]
- ...
内容来源：[文档名称列表]
```

3. 目标受众 / 教学深度 / 语言从用户描述或默认值确定（受众默认"相关领域的学习者"，深度默认中级，语言默认 zh-CN）

**注意：**
- 转换由你直接完成，**不要尝试调用脚本执行类工具**（`scripts/rag-to-requirement.py` 仅为规则化参考实现，其逻辑——提取主题 + 拼接 requirement——由你直接执行，效果等同且能结合内容理解）
- 若检索结果为空，直接用用户原始描述作为 `requirement`

### Phase 2: 构建 Generation Request

根据输入源构建请求体，**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requirement` | string | 是 | 教学主题描述，1-2 句话 |
| `pdfContent` | object | 否 | PDF 解析后的文本和图片 |
| `language` | string | 否 | `"zh-CN"` 或 `"en-US"`，默认 `"zh-CN"` |
| `enableWebSearch` | bool | 否 | 是否启用网络搜索，默认 false |
| `enableImageGeneration` | bool | 否 | 是否生成配图，默认 false |
| `enableVideoGeneration` | bool | 否 | 是否生成视频，默认 false |
| `enableTTS` | bool | 否 | 是否生成语音朗读，默认 false |
| `agentMode` | string | 否 | `"default"` 或 `"generate"`，默认 `"default"` |

场景适配：
- **场景 1（纯需求）**: `requirement` 直接使用用户描述
- **场景 2（RAG 结果）**: `requirement` 使用 Phase 1.1 构建的结果
- **场景 3（PDF）**: `requirement` 根据 PDF 提取的文本构建，`pdfContent` 填入解析结果

### Phase 3: 调用 OpenMAIC API

**优先方式**：通过 DocMind 注册的 MCP 工具直接调用 API。

**第一步：识别 HTTP 请求工具**
- 在你可用的 MCP 工具中，找到用于 HTTP 请求的工具
- 工具名称格式为 `mcp_{service_name}_{tool_name}`（如 `mcp_mcp_api_requester_make_request`）
- 通过工具描述（description）识别：寻找包含 "HTTP request"、"API"、"GET/POST" 等关键词的工具
- 如果找不到 HTTP 请求类 MCP 工具，则引导用户部署 mcp_api_requester（见 MCP 可用性检查）

**第二步：确定 Base URL 和认证 Header**

| 模式 | Base URL | 认证 Header |
|------|----------|-------------|
| 托管模式 | `https://open.maic.chat` | `Authorization: Bearer <access-code>` |
| 本地模式 | 用户提供的地址（DocMind 本地进程直接使用，无需替换） | 无 |

**第三步：Feature Detection（发送可选功能前）**

在发送生成请求前，先查询 `GET <BASE_URL>/api/health`（托管模式需带 auth header），检查返回的 `capabilities` 对象：

```json
{
  "status": "ok",
  "version": "...",
  "capabilities": {
    "webSearch": true,
    "imageGeneration": false,
    "videoGeneration": false,
    "tts": true
  }
}
```

- 只有当 `capabilities` 中某项为 `true` 时，才能在生成请求中将对应 feature flag 设为 `true`
- 如果服务器未返回 `capabilities`（旧版本），不要发送任何可选 feature flags

**第四步：发送 POST 请求**

使用识别到的 HTTP 请求工具发送请求。根据上面确定的模式和 URL 构造请求：

**托管模式**：
```json
{
  "url": "https://open.maic.chat/api/generate-classroom",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json",
    "Authorization": "Bearer <access-code>"
  },
  "body": {
    "requirement": "..."
  }
}
```

**本地模式**：
```json
{
  "url": "<BASE_URL>/api/generate-classroom",
  "method": "POST",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "requirement": "..."
  }
}
```

**MCP 工具不可用的处理**：

告知用户：

> 未检测到 `mcp_api_requester` MCP 服务。请先从 https://github.com/yryuu/mcp-api-requester 下载代码并部署，然后在 DocMind 中注册该 MCP 服务。

### Phase 4: 查询任务进度

**提交响应解析（必须）**：POST 提交成功后，从响应 JSON 中提取并**记住**：
- `jobId`（如 `ThQHsMlt0m`）
- `pollUrl`——**查询进度必须使用 pollUrl，其固定格式为 `{base_url}/api/generate-classroom/{jobId}`**
- 把 jobId 和 pollUrl 写入你的回复中，供后续用户询问进度时使用（不要只记在心里）

**查询端点（唯一正确路径）**：`GET {base_url}/api/generate-classroom/{jobId}`

> ⚠️ 不要尝试任何其他路径：`/api/jobs`、`/api/jobs/{id}`、`/api/classroom/generate`、`/api/restful/*` 等均**不存在**，会返回 404，纯属浪费时间。查询进度只有 `pollUrl` 这一个正确端点。

**第 1 次查询（提交后立即执行）**：
1. 调用 HTTP 请求工具 `GET {base_url}/api/generate-classroom/{jobId}`
2. 检查 `status`：
   - 如果 `succeeded` → 进入 Phase 5
   - 如果 `failed` → 报告错误并停止
   - 如果 `queued` 或 `running` → **停止查询，告知用户**：

     > 课程正在生成中（已生成 X/Y 个场景，progress 字段），预计需要 2-10 分钟。请稍后询问我查询进度。
     > Job ID: {jobId}

**用户询问进度时（第 2 次及之后查询）**：
1. 再次调用 `GET {base_url}/api/generate-classroom/{jobId}`（同一路径，jobId 不变）
2. 检查 `status`：
   - 如果 `succeeded` → 从响应 `result` 中提取 classroomId，进入 Phase 5
   - 如果 `failed` → 报告错误并停止
   - 如果仍在 `queued` 或 `running` → **停止查询，告知用户继续等待**（附上当前 progress 进度）：

     > 课程仍在生成中（当前进度 X%），请稍后再试。
     > Job ID: {jobId}

**重要规则**：
- 提交后只查询 **1 次**，不要连续轮询
- 用户询问进度时只查询 **1 次**，不要连续轮询
- 仅在 `status` 为 `succeeded` 或 `failed` 时才继续下一步——否则必须停止并告知用户等待
- 不要尝试重新提交 job——保持查询同一个 pollUrl（`{base_url}/api/generate-classroom/{jobId}`）
- **不要自己发明查询路径**：若不确定路径，就用 `{base_url}/api/generate-classroom/{jobId}`

### Phase 5: 返回结果

生成成功后，返回：

```
Classroom ID: <classroomId>
Classroom URL:
<BASE_URL>/classroom/<classroomId>
```

托管模式的 URL 格式：`https://open.maic.chat/classroom/<classroomId>`

> URL 必须以纯文本独占一行输出，不加粗、不加代码格式、不加 Markdown 链接。

## 错误处理

| 错误 | 含义 | 处理方式 |
|------|------|----------|
| 连接失败 | 网络不通或服务未启动 | 检查 Base URL 是否正确，服务是否启动 |
| 401 | 访问码无效（托管模式） | 告知用户到 open.maic.chat 检查或重新生成访问码 |
| 403 | 每日配额用尽（托管模式） | 告知每日 10 次限制，次日零点重置 |
| 500 | 服务器错误 | 建议稍后重试或切换到本地模式 |
| Provider 配置错误 | 模型/Provider/认证问题 | 引导用户检查 配置或联系管理员 |

## 多文档 → 课程集

当用户需要将多个文档/知识片段生成课程集时：

1. 收集所有文档内容
2. 为每个文档/主题分别生成 requirement
3. 通过 MCP 工具依次调用生成 API（不可并行，避免配额冲突）
4. 如果 MCP 工具不可用，告知用户先部署 mcp_api_requester（见 MCP 可用性检查）
5. 汇总返回所有 Classroom URL

## 注意事项

- requirement 构建由你直接完成（按 references/requirement-builder.md 模板），**不调用脚本执行类工具**；脚本仅做数据转换，不涉及网络调用
- **必须通过 DocMind 的 MCP 工具调用 OpenMAIC API**——不提供 curl 命令作为降级方案
- MCP 工具名称格式为 `mcp_{service_name}_{tool_name}`，根据描述识别 HTTP 请求工具
- 如果 MCP 工具未启用或不可用，告知用户先从 https://github.com/yryuu/mcp-api-requester 下载代码并部署，然后在 DocMind 中注册该 MCP 服务
- 单次生成任务预计 2-10 分钟，取决于内容复杂度和可选功能
- 托管模式（open.maic.chat）每天最多 10 次生成配额，独立于 Web UI 配额
- 如果用户在同一个 job 仍在运行时要求生成新课程，不要重复提交——先检查已有 job 状态
