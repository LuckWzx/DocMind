# DocMind Web 前端

DocMind 智能知识管理平台的前端项目，基于 Vue 3 + TypeScript + Vite 构建。

## 技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| 框架 | Vue 3 + Composition API | ^3.5 |
| 语言 | TypeScript | ~5.5 |
| 构建 | Vite | ^7 |
| UI 组件 | TDesign Vue Next | ^1.19 |
| 状态管理 | Pinia | ^3.0 |
| 路由 | Vue Router | ^4.5 |
| HTTP 客户端 | Axios | ^1.16 |
| 国际化 | vue-i18n | ^11.4 |
| 样式预处理 | Less | ^4.6 |
| Markdown 渲染 | marked + highlight.js + katex + mermaid | — |
| SSE 流式 | @microsoft/fetch-event-source | ^2.0 |

## 快速开始

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev        # 访问 http://localhost:5173

# 构建生产包
npm run build       # 仅构建
npm run build-with-types  # 构建 + 类型检查

# 预览构建结果
npm run preview

# 类型检查
npm run type-check

# 运行测试
npm test
```

开发服务器默认将 `/api` 和 `/files` 请求代理到后端（`http://localhost:3888`）。可通过环境变量 `VITE_DEV_PROXY_TARGET` 或 `FRONTEND_BACKEND_URL` 自定义代理目标。

## 项目结构

```
web/
├── public/                        # 静态资源（不经 Vite 处理）
│   ├── config.js                  # 运行时配置（API 地址等）
│   ├── favicon.ico
│   ├── weknora-widget.js          # Embed 窗口小部件
│   └── tdesign-icons/             # TDesign 图标本地副本（离线兼容）
├── src/
│   ├── api/                       # API 接口层（按模块拆分）
│   │   ├── auth/                  # 认证（登录/注册/OIDC）
│   │   ├── agent/                 # 智能体管理
│   │   ├── chat/                  # 聊天会话与流式消息
│   │   ├── knowledge-base/        # 知识库 CRUD、文件上传
│   │   ├── model/                 # LLM 模型与提供商配置
│   │   ├── chunker/               # 文档分块配置
│   │   ├── datasource/            # 数据源接入
│   │   ├── embed/                 # 嵌入式聊天
│   │   ├── initialization/        # 系统初始化
│   │   ├── mcp-service.ts         # MCP 服务管理
│   │   ├── organization/          # 组织管理
│   │   ├── skill/                 # 技能管理
│   │   ├── system/                # 系统管理
│   │   ├── tenant/                # 租户（成员/邀请/审计日志）
│   │   ├── wiki/                  # 知识库 Wiki
│   │   ├── retrieval.ts           # 检索配置
│   │   ├── vector-store.ts        # 向量存储配置
│   │   ├── web-search.ts          # 网页搜索
│   │   └── ...                    # 其他 API 模块
│   ├── assets/                    # 静态资源（图片/字体/主题 CSS）
│   │   ├── fonts/                 # 字体文件
│   │   ├── img/                   # 图标与插图
│   │   │   ├── im/                # IM 渠道图标
│   │   │   └── providers/         # 存储/向量/搜索提供商图标
│   │   └── theme/                 # 主题样式
│   ├── components/                # 公共组件
│   │   ├── GlobalCommandPalette/  # 全局命令面板（⌘K）
│   │   ├── chat/                  # 聊天相关组件
│   │   ├── credentials/           # 凭证管理
│   │   ├── css/                   # 组件样式（Markdown/代码高亮/引用等）
│   │   └── settings/              # 设置相关组件
│   ├── composables/               # 组合式函数（复用逻辑）
│   ├── config/                    # 应用配置
│   ├── directives/                # 自定义指令
│   ├── hooks/                     # 钩子函数
│   ├── i18n/                      # 国际化
│   │   └── locales/               # 语言包（zh-CN / en-US / ko-KR / ru-RU）
│   ├── router/                    # 路由配置与导航守卫
│   ├── stores/                    # Pinia 状态管理
│   ├── styles/                    # 全局样式
│   ├── types/                     # TypeScript 类型定义
│   ├── utils/                     # 工具函数
│   ├── views/                     # 页面组件
│   │   ├── agent/                 # 智能体管理页
│   │   ├── auth/                  # 登录/注册页
│   │   ├── chat/                  # 聊天页（含工具结果渲染等子组件）
│   │   ├── creatChat/             # 新建会话
│   │   ├── embed/                 # 嵌入式聊天页面
│   │   ├── integrations/         # 集成管理
│   │   ├── knowledge/             # 知识库（列表/详情/Wiki/设置）
│   │   ├── platform/              # 平台布局（侧边栏 + 主内容区）
│   │   ├── settings/              # 系统/租户/个人设置
│   │   └── system/                # 系统管理（运行时队列等）
│   ├── wailsjs/                   # Wails 桌面端桥接
│   ├── App.vue                    # 根组件（OIDC 回调/主题/TDesign 配置）
│   └── main.ts                    # 应用入口
├── embed.html                     # Embed 入口 HTML
├── index.html                     # SPA 入口 HTML
├── vite.config.ts                 # Vite 构建配置
├── tsconfig.json                  # TypeScript 配置
├── package.json
├── pnpm-workspace.yaml
├── Dockerfile                     # 生产镜像（Nginx）
├── docker-entrypoint.sh           # 容器入口脚本
└── nginx.conf                     # Nginx 配置（gzip/路由/API 代理）
```

## 核心功能模块

### 知识库管理
- 知识库 CRUD、置顶、复制
- 文件上传与解析（PDF / DOCX / MD / Excel / Web 等格式）
- 文档分块策略配置
- 标签管理与 Wiki 浏览
- 数据源接入（飞书/Notion/语雀/RSS 等）
- 向量存储与检索配置

### 智能对话
- 会话管理（创建/重命名/删除/分组）
- 流式消息（SSE）与停止生成
- Markdown 渲染 + 代码语法高亮 + 数学公式 + 流程图
- 工具调用结果展示（搜索/数据库/知识图谱/Web 抓取/Plan 等）
- @ 提及选择器（知识库/文件）
- 文件附件上传与预览
- 引用溯源与浮动引用卡片
- 追问建议

### 智能体
- 智能体创建、编辑、删除
- 系统提示词模板
- 模型/知识库绑定
- 分享设置与嵌入渠道（API / Web 嵌入 / IM 渠道）
- Agent 选择器与对话切换

### 模型管理
- LLM 提供商配置（OpenAI 兼容接口）
- 模型列表管理
- 模型测试与调试

### 设置中心
- 通用设置（语言/主题/自动更新）
- 模型配置
- 检索参数配置
- 存储引擎设置
- 解析引擎设置
- MCP 服务管理
- 网页搜索提供商
- 向量存储设置
- 租户信息与成员管理
- 用户个人信息

### 其他特性
- 全局命令面板（⌘K）：快速搜索与导航
- 主题切换：亮色 / 暗色 / 跟随系统
- 四语言国际化：中文 / English / 한국어 / Русский
- 新用户引导与上下文提示
- 系统初始化自动配置
- 租户邀请与管理
- 聊天记录管理
- 系统运行队列监控

## 开发说明

### 开发环境代理

开发模式下 Vite 自动将 `/api` 和 `/files` 代理到后端服务，默认目标为 `http://localhost:3888`。通过以下方式修改：

```bash
# 设置环境变量
export VITE_DEV_PROXY_TARGET=http://your-backend:3888
```

### 构建配置

- **多页面构建**：`index.html`（主 SPA） + `embed.html`（嵌入式聊天）
- **代码分割**：mermaid、marked/katex、highlight.js 独立为 vendor chunk
- **版本注入**：`__FRONTEND_VERSION__` 和 `__FRONTEND_COMMIT__` 在构建时注入

### TypeScript 类型

- `src/types/` 下定义业务类型（agent / chat / knowledge / model 等）
- `src/env.d.ts` 声明 `.vue` 模块类型和构建常量
- 接口对接时以 `src/types/` 下的类型定义为准

### API 对接

当后端接口就绪后，将 `src/api/` 下对应模块的 Mock 返回值替换为实际 HTTP 请求即可：

```typescript
// 修改前（Mock）
export async function listKnowledgeBases() {
  return Promise.resolve(mockKnowledgeBases)
}

// 修改后（实际调用）
export async function listKnowledgeBases() {
  return get('/v1/knowledge-bases')
}
```

### 国际化

- 语言包位于 `src/i18n/locales/`
- 当前支持：zh-CN、en-US、ko-KR、ru-RU
- 新增翻译时在各语言包中同步添加对应的 key

## 生产部署

```bash
# 1. 构建静态资源
npm run build        # 输出到 dist/

# 2. Docker 镜像
docker build -t docmind-web .
docker run -d -p 80:80 -e MAX_FILE_SIZE_MB=100 docmind-web
```

Nginx 配置默认开启 gzip 压缩（静态资源压缩后约 200-300KB），通过 `MAX_FILE_SIZE_MB` 环境变量控制上传文件大小限制（默认 50MB）。
