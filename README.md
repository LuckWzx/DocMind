# DocMind - 文档思维

> 📚 智能知识管理系统 - 将文档转化为可查询、可推理、持续进化的知识资产

## 📖 项目简介

DocMind 是一个现代化的智能知识管理平台，提供文档理解、语义检索、自主推理等核心功能。项目采用前后端分离架构，前端基于 Vue 3 + TypeScript + Vite 构建。

## 🏗️ 项目结构

```
DocMind/
├── README.md                          # 项目说明文档
├── web/                              # 前端项目（完整复刻WeKnora前端）
│   ├── package.json                  # 项目配置
│   ├── vite.config.ts               # Vite配置
│   ├── index.html                   # 入口HTML
│   └── src/
│       ├── main.ts                  # 入口文件
│       ├── App.vue                  # 根组件
│       ├── api/                     # API接口层（已预留Mock数据）
│       ├── views/                   # 页面组件（159个Vue组件）
│       ├── components/              # 公共组件
│       ├── stores/                  # Pinia状态管理
│       ├── router/                  # Vue Router路由配置
│       ├── types/                   # TypeScript类型定义
│       ├── utils/                   # 工具函数
│       ├── composables/             # 组合式函数
│       ├── i18n/                    # 国际化配置
│       └── assets/                  # 静态资源
├── backend/                         # 后端项目（待开发）
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

✅ **完整的前端框架** - Vue 3 + TypeScript + Vite  
✅ **159个Vue组件** - 完整复刻WeKnora前端  
✅ **模块化API设计** - 按功能模块分离，易于维护  
✅ **完整的类型定义** - TypeScript接口，类型安全  
✅ **Mock数据** - 开发阶段可独立运行  
✅ **清晰的替换标记** - 所有TODO位置都有注释  
✅ **现代化UI** - TDesign组件库，美观易用  
✅ **国际化支持** - 多语言配置  

### 后端对接指南

1. **API路径约定**：查看 `src/api/` 目录下的各个文件
2. **请求/响应格式**：参考 `src/types/` 目录下的类型定义
3. **替换Mock数据**：将API函数中的 `Promise.resolve(mockData)` 替换为实际的HTTP请求

## 📄 License

MIT License
