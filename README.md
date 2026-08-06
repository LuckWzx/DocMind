# DocMind - 文档思维

> 📚 智能知识管理系统 - 将文档转化为可查询、可推理、持续进化的知识资产

## 📖 项目简介

DocMind 是一个现代化的智能知识管理平台，提供文档解析、语义检索、自主推理等核心功能。项目采用前后端分离架构，并引入 **Eino** 框架实现强大的智能体（Agent）编排能力：

- **后端**：Go + Gin + GORM，引入 **Eino** 框架作为 AI 编排引擎，支持复杂的 Agent 流程编排与 RAG 管道设计
- **文档解析**：Python gRPC 微服务，支持 PDF / DOCX / Markdown / Excel / Web 等多格式
- **前端**：Vue 3 + TypeScript + Vite，TDesign 组件库

## 🏗️ 项目结构

```
DocMind/
├── cmd/                              # 应用入口
│   ├── agentdemo/                    # Agent 引擎最小 Demo（internal/agent 骨架验证）
│   │   └── main.go                   # 引擎端到端验证入口
│   └── server/
│       └── main.go                   # 服务启动入口
├── configs/                          # 配置文件
│   ├── config.yaml                   # 主配置（PostgreSQL / Redis / MinIO / DocReader）
│   └── config.yaml.example           # 配置示例
├── internal/                         # 内部模块（不对外暴露）
│   ├── api/                          # HTTP API 层
│   │   ├── router.go                 # 路由注册
│   │   └── v1/                       # API v1
│   │       ├── agent/                # AI Agent 模块
│   │       ├── auth/                 # 认证模块（注册/登录/刷新Token）
│   │       ├── chat/                 # 对话模块（SSE 流式问答）
│   │       │   ├── controller.go     # 会话 CRUD + SSE 流式问答
│   │       │   ├── routes.go         # 路由注册
│   │       │   └── sse_event.go      # SSE 事件类型常量池
│   │       ├── chunker/              # 分块配置模块
│   │       ├── initialization/       # 系统初始化
│   │       ├── knowledge/            # 知识条目（文件上传/解析/向量化）
│   │       ├── knowledgebase/        # 知识库（CRUD / FAQ / 文件导入）
│   │       ├── models/               # LLM 模型配置
│   │       ├── tag/                  # 标签模块（独立 CRUD）
│   │       ├── user/                 # 用户模块
│   │       └── vectorstore/          # 向量存储配置
│   ├── app/                          # 应用生命周期管理
│   │   └── app.go                    # 初始化、依赖注入、AutoMigrate、自动启动DocReader
│   ├── llm/                           # LLM 模型工厂层
│   │   ├── chat_model_factory.go       # Eino ChatModel 工厂（Agent 核心依赖）
│   │   ├── embedder_factory.go         # Eino Embedder 工厂（文档/查询向量化）
│   │   └── reranker_factory.go         # Rerank 模型工厂（检索结果重排）
│   ├── agent/                         # Agent 引擎层（ADK 封装，模块5）
│   │   ├── config.go                  # entity.AgentConfig → 引擎配置映射
│   │   ├── engine.go                  # 引擎接口与 ADK ChatModelAgent 封装
│   │   ├── runner.go                  # EventStream 事件展开层（ADK → 统一事件）
│   │   └── types.go                   # 统一事件类型与 RunRequest
│   ├── middleware/                    # 中间件
│   │   ├── auth.go                   # JWT 鉴权
│   │   ├── cors.go                   # 跨域
│   │   ├── logger.go                 # 请求日志
│   │   └── recovery.go              # 异常恢复
│   ├── model/                        # 数据模型
│   │   ├── dto/                      # 数据传输对象
│   │   │   ├── request/              # 请求 DTO（9个模块：auth / chunker / faq / knowledge / knowledge_base / model / tag / user / vector_store）
│   │   │   └── response/             # 响应 DTO（8个模块：chunker / faq / knowledge / knowledge_base / model / tag / user / vector_store）
│   │   └── entity/                   # 数据库实体（GORM）
│   │       ├── base.go               # BaseEntity（自增主键+软删除）
│   │       ├── user.go               # 用户
│   │       ├── agent.go              # Agent 配置
│   │       ├── session.go            # 对话会话
│   │       ├── message.go            # 对话消息
│   │       ├── knowledge_base.go     # 知识库
│   │       ├── knowledge.go          # 知识条目
│   │       ├── chunk.go              # 分块
│   │       ├── chunk_vector.go       # 分块向量
│   │       ├── faq.go                # FAQ 问答
│   │       ├── tag.go                # 标签
│   │       ├── model_config.go       # LLM 模型配置
│   │       ├── vector_store.go       # 向量存储
│   │       ├── system_setting.go     # 系统设置
│   │       ├── web_search_provider.go # 网页搜索
│   │       └── types.go              # 通用类型（JSON等）
│   ├── repository/                   # 数据访问层
│   │   ├── *_interface.go            # 仓储接口（25个文件，覆盖全部实体）
│   │   └── *_repository.go           # 仓储实现
│   └── service/                      # 业务逻辑层
│       ├── *_interface.go            # 服务接口（13个模块）
│       ├── *_service.go              # 服务实现
│       ├── model_service_http.go     # 模型服务 HTTP 工具（JSON/Multipart 请求、认证头、URL 拼接）
│       ├── model_service_ollama.go   # 模型服务 Ollama（状态、模型列表、异步下载、embed/chat）
│       ├── model_service_utils.go    # 模型服务工具（JSON 响应解析、类型转换、文件读取）
│       ├── knowledge_embedder.go     # 知识分块自动向量化（分批处理）
│       ├── vector_driver_postgres.go # pgvector 向量检索驱动
│       ├── image_storage_*.go        # 文档图片存储（MinIO / Noop）
│       ├── knowledge_image_pipeline.go  # 图片提取与URL替换管道
│       └── knowledge_pipeline_gateway*.go  # 知识管道网关（接口 + Mock）
├── pkg/                              # 公共工具包
│   ├── config/                       # 配置加载
│   ├── database/                     # 数据库驱动（PostgreSQL / MySQL / Redis）
│   ├── docreader/                    # 文档解析微服务（Python + gRPC）
│   │   ├── client/                   # Go gRPC 客户端
│   │   ├── models/                   # Python 数据模型
│   │   ├── ocr/                      # OCR 识别（Paddle / VLM）
│   │   ├── parser/                   # 文档解析器（PDF / DOCX / MD / Excel / Web / Image）
│   │   ├── proto/                    # Protobuf 定义
│   │   ├── splitter/                 # 文档分割器
│   │   ├── utils/                    # Python 工具函数
│   │   ├── config.py                 # Python 服务配置
│   │   ├── main.py                   # Python 服务入口（gRPC :50051）
│   │   └── pyproject.toml            # Python 项目配置
│   ├── errors/                       # 自定义错误码体系
│   ├── jwt/                          # JWT 令牌管理
│   ├── logger/                       # Zap 日志封装
│   ├── response/                     # 统一响应格式（分页/成功/错误）
│   └── utils/                        # 通用工具（字符串、时间）
├── scripts/                          # 脚本
│   ├── build.sh                      # 编译脚本
│   └── migrate.sql                   # 初始迁移 SQL
├── docs/                             # 设计文档
│   ├── API.md                        # API 文档
│   ├── ARCHITECTURE.md               # 架构文档
│   ├── DEVELOPMENT.md                # 开发指南
│   ├── swagger.yaml / swagger.json / swagger.md # Swagger 规范
│   ├── agent沟通注意事项.md            # Agent 对话设计要点
│   ├── 甲.md / 乙.md                 # 数据库结构体设计 & 数据流交互
│   ├── 乙模块结构体评审.md             # 乙模块结构体评审
│   ├── 阶段一.md / 阶段二.md          # 分阶段开发规划
│   ├── 阶段二排期与逻辑.md             # 阶段二排期与核心逻辑
│   ├── 知识库api.md                   # 知识库 API 规范
│   ├── 标签crud.md                   # 标签 CRUD 设计
│   ├── 模型集成.md                   # LLM 模型集成方案
│   └── 思维导图.md                   # 系统思维导图
├── web/                              # 前端项目（Vue 3 + TypeScript）
│   ├── src/
│   │   ├── api/                      # API 接口层（Mock 预留）
│   │   ├── views/                    # 页面组件
│   │   ├── components/               # 公共组件
│   │   ├── stores/                   # Pinia 状态管理
│   │   ├── router/                   # Vue Router 路由
│   │   ├── types/                    # TypeScript 类型
│   │   ├── utils/                    # 工具函数
│   │   └── composables/              # 组合式函数
│   ├── package.json
│   └── vite.config.ts
├── go.mod                            # Go 模块定义
├── go.sum                            # Go 依赖锁定
├── Makefile                          # 构建命令
└── README.md
```

## 🚀 快速开始

### 环境要求

**后端：**
- Go >= 1.23
- PostgreSQL >= 15
- Redis >= 7（可选）
- Python >= 3.10（docreader 文档解析服务）

**前端：**
- Node.js >= 18
- npm 或 pnpm

### 启动后端

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env 填入数据库连接信息

# 2. 启动服务（自动执行数据库迁移）
go run cmd/server/main.go
```

访问 http://localhost:3888 ，Swagger 文档 http://localhost:3888/swagger/index.html

### 启动前端

```bash
cd web
npm install
npm run dev
```

访问 http://localhost:5173

## 🔌 API接口说明

### 接口预留方式

项目采用 **Mock数据 + 接口定义** 的方式预留后端接口：

1. **类型定义**：在 `src/types/` 目录下定义完整的TypeScript接口
2. **API层**：在 `src/api/` 目录下创建API函数，当前返回Mock数据
3. **注释标记**：所有需要替换的地方都有 `// TODO: 替换为实际API调用` 注释

### 如何对接后端

当后端接口就绪后，只需修改对应的API文件：

```typescript
// 修改前（Mock数据）
export async function listKnowledgeBases() {
  console.log('listKnowledgeBases')
  return Promise.resolve(mockKnowledgeBases)
}

// 修改后（实际API调用）
export async function listKnowledgeBases() {
  return get('/v1/knowledge-bases')
}
```

### API模块清单

| 模块 | 功能 | 文件路径 |
|------|------|----------|
| **知识库** | 后端 CRUD 已实现（创建/列表/详情/更新/删除/置顶） | `internal/api/v1/knowledgebase/` |
| **知识条目** | 文件上传、DocReader 解析、Markdown 分块、状态追踪 | `internal/api/v1/knowledge/` |
| **FAQ** | 问答对管理、批量导入导出 | `internal/api/v1/knowledgebase/` |
| **标签** | 标签独立 CRUD，支持按知识库筛选 | `internal/api/v1/tag/` |
| **聊天** | 会话管理、消息收发、SSE 流式响应 | `internal/api/v1/chat/` |
| **Agent** | 智能体 CRUD、复制、内置 Agent 种子数据（快速问答） | `internal/api/v1/agent/` |
| **模型** | LLM / Embedding / Rerank / VLLM / ASR 多类型模型 CRUD、凭据管理、连通性探测、调试调用 | `internal/api/v1/models/` |
| **向量存储** | PostgreSQL（pgvector）向量引擎配置与语义检索 | `internal/api/v1/vectorstore/` |
| **认证** | 登录、注册、Token 刷新、登出 | `internal/api/v1/auth/` |
| **分块** | 多策略文档分块（heading / heuristic / legacy / auto） | `internal/api/v1/chunker/` |
| **初始化** | 系统初始化配置向导、Ollama 状态/下载/模型列表、供应商列表 | `internal/api/v1/initialization/` |

## 🛠️ 技术栈

**后端：**
- **框架**: Gin（HTTP 路由）
- **AI 编排**: **Eino**（Agent 引擎、RAG 管道、Graph 编排、Tool Calling）
- **ORM**: GORM（PostgreSQL + pgvector 向量扩展）
- **向量检索**: pgvector（IVFFlat / HNSW 索引，Cosine / L2 / IP 相似度）
- **认证**: JWT（双 Token 机制：Access + Refresh）
- **日志**: Zap（结构化日志 + 请求级上下文）
- **文档**: Swagger / OpenAPI
- **RPC**: gRPC + Protobuf（docreader Python 微服务）
- **文档解析**: Python（PaddleOCR / VLM / MarkItDown）
- **图片存储**: MinIO（文档内嵌图片持久化，Markdown URL 自动替换）

**前端：**
- **框架**: Vue 3 + Composition API
- **语言**: TypeScript
- **构建**: Vite
- **状态管理**: Pinia
- **UI组件**: TDesign
- **路由**: Vue Router
- **HTTP客户端**: Axios
- **国际化**: vue-i18n

## 📝 开发说明

### 项目特点

✅ **知识库 CRUD** — 完整的知识库增删改查 + 置顶，基于用户隔离  
✅ **文档导入解析** — 上传 PDF/DOCX/MD/Excel/Web → DocReader 解析 → 多策略智能分块  
✅ **向量语义检索** — pgvector 向量引擎，Cosine/L2/IP 相似度，IVFFlat/HNSW 索引  
✅ **图片持久化** — 文档内嵌图片自动上传 MinIO，Markdown 引用自动替换为公网 URL  
✅ **FAQ 管理** — 问答对批量导入/导出/增删改查  
✅ **标签体系** — 知识库标签管理，支持按标签筛选  
✅ **Eino Agent 编排** — Eino ChatModel + Embedder 工厂，ReAct 推理循环，Tool Calling 支持  
✅ **SSE 流式对话** — 知识问答流式响应，检索结果引用溯源  
✅ **15 张数据表** — AutoMigrate 自动迁移，PostgreSQL JSONB + pgvector 支持  
✅ **多模型管理** — 6 个供应商（OpenAI / 阿里云 / SiliconFlow / 智谱 / Jina / 自定义），5 类模型统一管理，凭据脱敏存储，Ollama 本地模型下载与管理  
✅ **11 个 API 模块** — 按功能模块分离，完整的前后端类型定义  
✅ **Go 后端** — Gin + GORM 分层架构（API → Service → Repository），Swagger 文档，JWT 双 Token  
✅ **文档解析服务** — Python gRPC 微服务，支持 PDF/DOCX/MD/Excel/Web/Image  
✅ **完整的前端框架** — Vue 3 + TypeScript + Vite + TDesign UI  
✅ **国际化支持** — vue-i18n 多语言配置  

### 后端对接指南

1. **API路径约定**：查看 `src/api/` 目录下的各个文件
2. **请求/响应格式**：参考 `src/types/` 目录下的类型定义
3. **替换Mock数据**：将API函数中的 `Promise.resolve(mockData)` 替换为实际的HTTP请求

## 📄 License

MIT License
