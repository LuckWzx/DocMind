# DocMind - 文档思维

> 📚 智能知识管理系统 - 将文档转化为可查询、可推理、持续进化的知识资产

## 📖 项目简介

DocMind 是一个现代化的智能知识管理平台，提供文档理解、语义检索、自主推理等核心功能。项目采用前后端协同架构，当前仓库同时包含前端应用与 Go 后端服务。

## 🏗️ 项目结构

```
DocMind/
├── cmd/                             # Go 服务启动入口
├── configs/                         # 配置模板
├── docs/                            # 架构、接口与阶段文档
├── internal/                        # 后端业务代码
├── migrations/                      # 数据库迁移脚本
├── pkg/                             # 公共基础库
├── scripts/                         # 构建与辅助脚本
├── web/                             # Vue 3 前端项目
│   ├── package.json                 # 前端依赖与脚本
│   ├── index.html                   # 主入口 HTML
│   ├── embed.html                   # 嵌入式聊天入口
│   └── src/
│       ├── api/                     # 前端 API 封装
│       ├── views/                   # 页面视图
│       ├── components/              # 公共组件
│       ├── stores/                  # Pinia 状态管理
│       ├── router/                  # Vue Router 路由
│       ├── types/                   # TypeScript 类型定义
│       ├── utils/                   # 工具函数
│       ├── composables/             # 组合式函数
│       └── i18n/                    # 国际化配置
├── go.mod                           # Go 模块定义
└── README.md
```

## 🚀 快速开始

### 环境要求

- Node.js >= 18
- npm 或 pnpm

### 安装依赖

```bash
cd web
npm install
```

### 启动开发服务器

```bash
npm run dev
```

访问 http://localhost:5173

## 🔌 API接口说明

### 接口预留方式

项目当前采用 **真实接口优先 + 局部占位实现补齐** 的方式推进前后端协同开发：

1. **类型定义**：在 `src/types/` 目录下定义完整的 TypeScript 接口
2. **API层**：在 `src/api/` 目录下封装统一请求，已逐步接入 `/api/v1/*` 后端接口
3. **迭代方式**：已落地模块直接调用真实接口，未落地模块保留占位实现或临时 mock

### 如何对接后端

当前仓库已经包含 Go 后端服务骨架与部分基础接口（如认证、用户等）。对于尚未落地的业务模块，可继续在对应 API 文件中保留占位实现，待后端补齐后再切换到真实接口：

```typescript
// 占位实现
export async function listKnowledgeBases() {
  console.log('listKnowledgeBases')
  return Promise.resolve(mockKnowledgeBases)
}

// 接入真实接口
export async function listKnowledgeBases() {
  return get('/v1/knowledge-bases')
}
```

### API模块清单

| 模块 | 功能 | 文件路径 |
|------|------|----------|
| **知识库** | CRUD、文件上传、标签管理 | `src/api/knowledge-base/index.ts` |
| **聊天** | 会话管理、消息收发、流式响应 | `src/api/chat/index.ts` |
| **Agent** | 创建、编辑、配置 | `src/api/agent/index.ts` |
| **模型** | LLM提供商配置、模型管理 | `src/api/model/index.ts` |
| **认证** | 登录、登出、Token管理 | `src/api/auth/index.ts` |

## 🛠️ 技术栈

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

✅ **前后端同仓协作** - Vue 3 前端 + Go 后端
✅ **完整的前端框架** - Vue 3 + TypeScript + Vite
✅ **丰富的界面与业务模块** - 覆盖知识库、聊天、Agent、设置等核心页面
✅ **模块化API设计** - 按功能模块分离，易于维护  
✅ **完整的类型定义** - TypeScript接口，类型安全  
✅ **后端骨架已落地** - 已包含配置、日志、数据库、认证与用户模块
✅ **清晰的替换标记** - 所有TODO位置都有注释  
✅ **现代化UI** - TDesign组件库，美观易用  
✅ **国际化支持** - 多语言配置  

### 后端对接指南

1. **API路径约定**：查看 `src/api/` 目录下的各个文件
2. **请求/响应格式**：参考 `src/types/` 目录下的类型定义
3. **补齐业务接口**：将占位实现逐步替换为实际的 HTTP 请求

## 📄 License

MIT License
