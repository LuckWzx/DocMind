# mcp-api-requester 部署与 DocMind 接入指南

> 目标：让 `openmaic-classroom`（互动课程生成）技能可用。
> 该技能依赖外部 MCP Server —— **mcp-api-requester**（HTTP API 请求器，来源：https://github.com/yryuu/mcp-api-requester），
> 通过它调用 OpenMAIC 课程生成 API（https://open.maic.chat，需访问码）。

---

## 一、整体流程

```
1. 获取 mcp-api-requester 源码/包（GitHub）
2. 在 DocMind 中添加 MCP 服务（stdio 方式，走 API）
3. 测试连接，确认 make_request 工具可用
4. Agent 配置启用 MCP + openmaic-classroom 技能
5. 提问验证端到端
```

---

## 二、获取 mcp-api-requester（三选一）

> ⚠️ 本机实测：**GitHub 直连不通**（`npx -y github:yryuu/mcp-api-requester` 报 exit 128，
> WebFetch github.com 超时）。请按下面任选一种可用的方式。

### 方案 A：配置代理后 npx 启动（有代理时推荐）

1. 确认代理可用（如 Clash 默认端口 7890）
2. 配置 npm/git 走代理后，DocMind 添加服务时直接填：

```json
{
  "command": "npx",
  "args": ["-y", "github:yryuu/mcp-api-requester"]
}
```

### 方案 B：GitHub 镜像下载 + 本地构建（无代理时推荐）

```powershell
# 1. 用 ghproxy 镜像克隆（GitHub 直连不通时）
git clone https://ghproxy.com/https://github.com/yryuu/mcp-api-requester
# 备选镜像：https://mirror.ghproxy.com/ 、https://gh-proxy.com/

# 2. 构建
cd mcp-api-requester
npm install
npm run build

# 3. 启动命令（记下绝对路径，后面添加服务要用）
#    node D:/GoLang/DocMind/../mcp-api-requester/dist/index.js
```

### 方案 C：下载 tarball 解压（GitHub 页面打不开时）

浏览器访问镜像 URL 直接下载压缩包：

```
https://ghproxy.com/https://github.com/yryuu/mcp-api-requester/archive/refs/heads/main.tar.gz
```

解压后同样执行 `npm install && npm run build`。

> 验证是否可用：`node <绝对路径>/dist/index.js` 启动后无报错即正常（MCP server 启动后等待 stdin 输入，不会退出）。

---

## 三、在 DocMind 中添加 MCP 服务

> MCP 服务**按用户隔离**：A 用户添加的服务 B 用户看不到，需各自添加。
> 前端设置页只支持远程（sse / http-streamable），**stdio 必须走 API**。

### 方式一：API 添加（stdio / 本地 npx 或 node 启动）

**npx 方式（已配置代理）：**

```powershell
$token = "<你的登录token>"
$body = @{
  name           = "mcp_api_requester"
  description    = "HTTP API 请求器（openmaic-classroom 技能依赖）"
  transport_type = "stdio"
  stdio_config   = @{ command = "npx"; args = @("-y", "github:yryuu/mcp-api-requester") }
  advanced_config = @{ timeout = 120; retry_count = 1; retry_delay = 1 }  # 首次下载慢，超时给足
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Uri "http://localhost:3888/api/v1/mcp-services" -Method Post `
  -Headers @{ Authorization = "Bearer $token" } -ContentType "application/json" -Body $body
```

**本地 node 方式（推荐，构建好后最稳）：**

```powershell
$body = @{
  name           = "mcp_api_requester"
  description    = "HTTP API 请求器（openmaic-classroom 技能依赖）"
  transport_type = "stdio"
  stdio_config   = @{ command = "node"; args = @("D:/mcp-api-requester/dist/index.js") }  # 改成本机绝对路径
  advanced_config = @{ timeout = 30 }
} | ConvertTo-Json -Depth 5

Invoke-RestMethod -Uri "http://localhost:3888/api/v1/mcp-services" -Method Post `
  -Headers @{ Authorization = "Bearer $token" } -ContentType "application/json" -Body $body
```

> ⚠️ **服务名必须叫 `mcp_api_requester`**：工具名会自动变成 `mcp_mcp_api_requester_make_request`，
> 与 SKILL.md 中技能描述的工具名完全一致，模型才能对上号。

### 方式二：前端添加（仅远程服务）

设置页 → MCP 服务 → 添加：填名称 `mcp_api_requester`、传输方式 SSE 或 HTTP-Streamable、URL。
（仅当你能把 mcp-api-requester 以远程方式部署时适用；默认它是 stdio 的，一般用方式一）

---

## 四、测试连接

```powershell
$token = "<你的登录token>"
Invoke-RestMethod -Uri "http://localhost:3888/api/v1/mcp-services/<服务ID>/test" -Method Post `
  -Headers @{ Authorization = "Bearer $token" } -ContentType "application/json" -Body "{}"
```

成功响应中 `data.tools` 应包含 **`make_request`**（url / method / headers / params / cookies / body 参数）。

> 说明：连接 DocMind 时 **auth_config 留空**即可 —— OpenMAIC 的访问码是 LLM 调用时通过
> `make_request` 的 `headers` 动态传入的，不是 MCP 服务连接层认证。

---

## 五、Agent 启用 MCP + openmaic-classroom 技能

给目标 Agent 的 `config` 增加（前端 Agent 编辑页或 `PUT /api/v1/agents/:id`）：

```json
{
  "mcp_selection_mode": "manual",
  "mcp_services": ["<服务ID>"],
  "skills_selection_mode": "manual",
  "selected_skills": ["openmaic-classroom"]
}
```

> `openmaic-classroom` 默认"暂不启用"，加入 SelectedSkills 即启用。
> 若希望所有已启用 MCP 服务都挂载，`mcp_selection_mode` 可填 `"all"`（无需列 ID）。

---

## 六、端到端验证

1. 打开对话，选择配置好的 Agent（smart-reasoning 模式）
2. 提问：**"把知识库里 XX 文档做成互动课件"**
3. 预期 SSE 事件流：
   - `skill` 工具加载 openmaic-classroom 技能
   - `mcp_mcp_api_requester_make_request` 工具声明 → 调用 OpenMAIC API → 结果回填
   - 最终回答给出 Classroom URL（托管模式：`https://open.maic.chat/classroom/<id>`）

> 需要 OpenMAIC 访问码（`sk-` 开头）时，模型会询问你，直接粘贴即可（托管模式每天 10 次配额）。
> 本地模式：自行部署 OpenMAIC 后把地址告诉模型（DocMind 是本地进程，直接用 `localhost` 地址，无需任何替换）。

---

## 七、常见问题排查

| 现象 | 原因 | 处理 |
|---|---|---|
| 测试报 `transport closed` / `transport error` | 子进程启动后立即退出：多为 GitHub 拉取失败（exit 128）或 node 路径错误 | 按第二节方案 B/C 本地构建，改用 `node <绝对路径>` 启动 |
| 测试超时（10s 默认） | npx 首次下载 npm 包较慢 | `advanced_config.timeout` 调到 120 |
| `make_request` 未出现在工具列表 | 服务名不叫 `mcp_api_requester`，或该服务未启用 | 改名/`enabled: true` |
| Agent 说"未检测到 mcp_api_requester" | 该 Agent 的 `mcp_selection_mode` 未配置或服务 ID 不对 | 检查 Agent config 的 `mcp_services` |
| 注册账号报错 | 邮箱域名需有 MX 记录（test.com 等必失败）；密码需含大写+小写+数字 | 用真实邮箱（如 qq.com）；密码如 `Abc123456` |

---

## 八、参考

- mcp-api-requester 仓库：https://github.com/yryuu/mcp-api-requester
- DocMind MCP 管理接口：`internal/api/v1/mcp/controller.go`（Swagger: http://localhost:3888/swagger/index.html）
- 技能文件：`configs/skills/openmaic-classroom/SKILL.md`（已做 DocMind 适配）
- 本地联调测试 Server：`scripts/mcp_test_server.py`（纯标准库，验证 MCP 链路可用）
