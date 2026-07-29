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
│   └── server/
│       └── main.go                   # 服务启动入口
├── configs/                          # 配置文件
│   ├── config.yaml                   # 主配置
│   └── config.yaml.example           # 配置示例
├── internal/                         # 内部模块（不对外暴露）
│   ├── api/                          # HTTP API 层
│   │   ├── router.go                 # 路由注册
│   │   └── v1/                       # API v1
│   │       ├── auth/                 # 认证模块
│   │       ├── knowledgebase/        # 知识库模块
│   │       └── user/                 # 用户模块
│   ├── app/                          # 应用生命周期管理
│   │   └── app.go                    # 初始化、依赖注入、AutoMigrate
│   ├── middleware/                    # 中间件
│   │   ├── auth.go                   # JWT 鉴权
│   │   ├── cors.go                   # 跨域
│   │   ├── logger.go                 # 请求日志
│   │   └── recovery.go              # 异常恢复
│   ├── model/                        # 数据模型
│   │   ├── dto/                      # 数据传输对象
│   │   │   ├── request/              # 请求 DTO
│   │   │   │   └── knowledge_base.go # 知识库请求 DTO
│   │   │   └── response/             # 响应 DTO
│   │   │       └── knowledge_base.go # 知识库响应 DTO
│   │   └── entity/                   # 数据库实体（GORM）
│   │       ├── base.go               # BaseEntity（自增主键+软删除）
│   │       ├── user.go               # 用户
│   │       ├── refresh_token.go      # 刷新令牌
│   │       ├── knowledge_base.go     # 知识库
│   │       ├── knowledge.go          # 知识条目
│   │       ├── chunk.go              # 分块
│   │       ├── session.go            # 会话
│   │       ├── message.go            # 消息
│   │       ├── tag.go                # 标签
│   │       ├── model_config.go       # 模型配置
│   │       ├── web_search_provider.go # 网页搜索
│   │       └── types.go              # 通用类型（JSON等）
│   ├── repository/                   # 数据访问层
│   │   ├── user_interface.go         # 用户仓库接口
│   │   ├── user_repository.go        # 用户仓库实现
│   │   ├── refresh_token_interface.go
│   │   ├── refresh_token_repository.go
│   │   ├── knowledge_base_interface.go
│   │   └── knowledge_base_repository.go
│   └── service/                      # 业务逻辑层
│       ├── auth_interface.go         # 认证服务接口
│       ├── auth_service.go           # 认证服务实现
│       ├── user_interface.go         # 用户服务接口
│       ├── user_service.go           # 用户服务实现
│       ├── knowledge_base_interface.go
│       └── knowledge_base_service.go
├── pkg/                              # 公共工具包
│   ├── config/                       # 配置加载
│   ├── database/                     # 数据库驱动（PostgreSQL / MySQL / Redis）
│   ├── docreader/                    # 文档解析服务（Python + gRPC）
│   │   ├── client/                   # Go gRPC 客户端
│   │   ├── models/                   # Python 数据模型
│   │   ├── ocr/                      # OCR 识别（Paddle / VLM）
│   │   ├── parser/                   # 文档解析器（PDF / DOCX / MD / Excel / Web）
│   │   ├── proto/                    # Protobuf 定义
│   │   ├── splitter/                 # 文档分割器
│   │   ├── utils/                    # Python 工具函数
│   │   ├── config.py                 # Python 服务配置
│   │   ├── main.py                   # Python 服务入口
│   │   └── pyproject.toml            # Python 项目配置
│   ├── errors/                       # 自定义错误
│   ├── jwt/                          # JWT 令牌管理
│   ├── logger/                       # Zap 日志封装
│   ├── response/                     # 统一响应格式
│   └── utils/                        # 通用工具（字符串、时间）
├── scripts/                          # 脚本
│   ├── build.sh                      # 编译脚本
│   └── migrate.sql                   # 初始迁移 SQL
├── docs/                             # 设计文档
│   ├── API.md                        # API 文档
│   ├── ARCHITECTURE.md               # 架构文档
│   ├── DEVELOPMENT.md                # 开发指南
│   ├── swagger.yaml                  # Swagger 规范
│   ├── 甲.md                         # 数据库结构体设计
│   └── 乙.md                         # 数据流与模块交互
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
| **知识库** | 后端 CRUD 已实现（创建/列表/详情/更新/删除/置顶），前端 Mock 预留 | `internal/api/v1/knowledgebase/` |
| **聊天** | 会话管理、消息收发、流式响应 | `src/api/chat/index.ts` |
| **Agent** | 创建、编辑、配置 | `src/api/agent/index.ts` |
| **模型** | LLM提供商配置、模型管理 | `src/api/model/index.ts` |
| **认证** | 登录、登出、Token管理 | `src/api/auth/index.ts` |

## 🛠️ 技术栈

**后端：**
- **框架**: Gin（HTTP 路由）
- **AI 编排**: **Eino**（Agent 引擎、RAG 管道、Graph 编排）
- **ORM**: GORM（PostgreSQL）
- **认证**: JWT（双 Token 机制）
- **日志**: Zap
- **文档**: Swagger / OpenAPI
- **RPC**: gRPC + Protobuf（docreader）
- **文档解析**: Python（PaddleOCR / VLM / MarkItDown）

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

✅ **Eino 智能体编排** — 基于 Eino 的 Graph 和 Chain 机制，实现灵活的 Agent 推理循环与工具调用  
✅ **Go 后端** — Gin + GORM 分层架构，Swagger 文档，JWT 双 Token  
✅ **文档解析服务** — Python gRPC 微服务，支持 PDF/DOCX/MD/Excel/Web  
✅ **10 张数据表** — AutoMigrate 自动迁移，PostgreSQL JSONB 支持  
✅ **完整的前端框架** — Vue 3 + TypeScript + Vite  
✅ **模块化API设计** — 按功能模块分离，易于维护  
✅ **完整的类型定义** — TypeScript 接口，类型安全  
✅ **Mock数据** — 开发阶段可独立运行  
✅ **清晰的替换标记** — 所有 TODO 位置都有注释  
✅ **现代化UI** — TDesign组件库，美观易用  
✅ **国际化支持** — 多语言配置  

### 后端对接指南

1. **API路径约定**：查看 `src/api/` 目录下的各个文件
2. **请求/响应格式**：参考 `src/types/` 目录下的类型定义
3. **替换Mock数据**：将API函数中的 `Promise.resolve(mockData)` 替换为实际的HTTP请求

## 📄 License

MIT License
