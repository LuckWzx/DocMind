# QA 增强检索实现思路

> 目标：在文件上传解析流程中，为每个分片（chunk）用 LLM 生成"用户可能问的问题"并与之关联存储，检索时提升召回率（Recovery Rate）。

## 1. 背景与目标

当前 RAG 检索链路为：用户提问 → 向量检索 → 重排 → 生成答案。存在一个经典痛点：

- 用户口语化提问（如 *"怎么配置XXX？"*）
- chunk 正文是陈述式内容（如 *"配置步骤如下…"*）

两者在向量空间中的距离较远，导致**召回率低**、答案不完整甚至答非所问。

**解决思路（QA 增强检索 + 混合检索）**：解析时用 LLM 为每个 chunk 生成 3~5 个"用户最可能问的问题"并写入 chunk 元数据；**问题层不向量化**，而是对问题字段建立 pg_search BM25 稀疏索引（内置 Jieba 中文分词），检索时执行「BM25 关键词检索 + pgvector 向量检索」双路召回，经 RRF（Reciprocal Rank Fusion）融合后统一重排，显著提升口语化提问的召回率。

## 2. 整体设计

```
上传文件
  │
  ▼
StageUpload 暂存 ──► ParseDocument 解析（DocReader）
  │
  ▼
创建 Chunk（逐条入库）
  │
  ▼
┌─ GenerateQuestions：批量调 LLM 为每个 chunk 生成 3~5 个问题 ─┐
│   · 写入 chunk.Metadata.GeneratedQuestions                    │
│   · 失败降级：只记日志，不阻塞主流程                            │
└──────────────┬───────────────────────────────┬───────────────┘
               ▼                               ▼
      问题字段建 BM25 索引                向量化纯正文（标题+上下文+内容）
      （pg_search + Jieba 分词）         写入 ChunkVector（pgvector）
               │                               │
               └───────────┬───────────────────┘
                           ▼
              检索时双路召回（BM25 + 向量）→ RRF 融合 → 重排 → 生成答案
```

核心原则：**向量检索链路零改动**，新增 BM25 稀疏检索分支，与向量结果在 `node_rerank` 之前完成 RRF 融合。

## 3. 数据存储设计

**复用 `chunks.metadata` JSON 列，零建表、零迁移。**

在 `ChunkMetadata` 结构体（`internal/model/entity/chunk.go`）中新增字段：

```go
// ChunkMetadata 文档分块的结构化元数据
type ChunkMetadata struct {
    DocTitle          string   `json:"doc_title,omitempty"`
    HeadingPath       []string `json:"heading_path,omitempty"`
    ContextHeader     string   `json:"context_header,omitempty"`
    // ... 现有字段
    GeneratedQuestions []string `json:"generated_questions,omitempty"` // 新增：LLM 生成的候选问题
}
```

### 为什么存 metadata 而不是新表

| 对比项 | metadata JSON | 独立表（如 knowledge_questions） |
|---|---|---|
| 生命周期 | 随 chunk 删除/重建自动清理 | 需手动级联删除 |
| 重解析 | `processKnowledge` 先删后建 chunk，问题自动重建 | 需额外清理逻辑 |
| 检索关联 | 问题与 chunk 天然一体，无需 join | 需 join 或二次查询 |
| 改动量 | 一个字段 | 实体 + repo + 迁移 |

问题本质是 chunk 的附属物，随 chunk 走最合理。

## 4. 生成时机与流程

挂载点：`internal/service/knowledge_base_service.go` 的 `processKnowledge` 中，**chunk 创建完成之后、向量化（BuildEmbeddings）之前**。

```go
// 伪代码：processKnowledge 中的新增步骤
chunks := 已创建的 chunk 列表

// 4.1 生成问题（可选开关控制）
if shouldGenerateQuestions(kb, item.ProcessConfig) {
    questionsMap, err := generateQuestions(ctx, llmModelID, chunks)
    if err != nil {
        // 失败降级：只记日志，不调 markKnowledgeFailed
        logger.Warnf("问题生成失败，跳过: %v", err)
    } else {
        // 4.2 写回 metadata 并更新
        for i, chunk := range chunks {
            meta := chunk.MetadataStruct()
            meta.GeneratedQuestions = questionsMap[chunk.ID]
            chunk.Metadata = marshalJSON(meta)
        }
        batchUpdateChunkMetadata(chunks)
    }
}

// 4.3 向量化保持纯正文（问题不参与 embedding，节省 token 成本）
// 向量化文本 = 标题 + 上下文标题 + 内容（复用 Chunk.EmbeddingContent 思路）
// 问题字段仅由 pg_search BM25 索引消费（见第 6 章）
```

### 关键约束

1. **失败降级**：问题生成是增强项，任何失败（LLM 超时、JSON 解析失败、配额不足）只记日志，**绝不**调用 `markKnowledgeFailed` 阻塞索引主流程。
2. **开关控制**：默认关闭，通过 `kb.IndexingStrategy` 或 `item.ProcessConfig`（如 `generate_questions: true`）开启，避免无谓的 token 消耗。
3. **异步友好**：`processKnowledge` 本身就是 `go` 协程异步执行，生成耗时不影响上传接口响应。

## 5. LLM 调用封装

现有 `ChatModelFactory.CreateChatModel(ctx, modelID)`（`internal/service/chat_model_factory.go`）可直接按模型 ID 创建 eino ChatModel，无需新依赖。

两种封装方式（二选一）：

**方式 A：注入 ChatModelFactory 到 knowledgeBaseService**

```go
// knowledgeBaseService 增加字段
chatModelFactory *ChatModelFactory

// 生成步骤内部：
chatModel, err := s.chatModelFactory.CreateChatModel(ctx, modelID)
// 拼接 prompt：根据以下文档片段生成 3~5 个用户最可能问的问题，返回 JSON 数组
// 调用 chatModel.Generate(ctx, messages)，解析返回的 JSON
```

**方式 B：扩展 KnowledgePipelineGateway（推荐，与现有 gateway 模式一致）**

```go
// KnowledgePipelineGateway 增加方法
GenerateQuestions(ctx context.Context, modelID string, chunks []*entity.Chunk) (map[uint][]string, error)
```

并提供 Mock 实现（`knowledge_pipeline_gateway_mock.go`），便于无 LLM 环境下测试。

### Prompt 设计要点

- 要求输出**严格 JSON 数组**，便于解析（如 `["问题1","问题2",...]`）
- 约束"只生成文档片段**能明确回答**的问题"，过滤离题/重复问题
- 限制数量（3~5 个），控制 token 成本
- 一次请求可喂多个 chunk（批量），减少调用次数

## 6. 召回增强机制（核心）：BM25 稀疏检索 + RRF 融合

**问题层不向量化**，对 `chunks.metadata->>'generated_questions'` 建 pg_search BM25 索引（底层 Tantivy 引擎，真 BM25 评分），检索时与向量检索双路召回、RRF 融合。

### 6.1 建索引（一次性，随库初始化幂等执行）

```sql
CREATE EXTENSION IF NOT EXISTS pg_search;

CREATE INDEX idx_chunks_questions_bm25 ON chunks
USING paradedb (
    id,
    (COALESCE(metadata->>'generated_questions', '')::pdb.jieba)
)
WITH (key_field = 'id');
```

- `pdb.jieba`：官方内置中文分词器（词典 + 统计模型），正确切分"怎么配置XXX"这类词边界；若性能敏感可降级为 `chinese-compatible`（简单中日韩切分）
- `key_field` 必须指向表主键（`id`）
- 过滤条件（`user_id`、`knowledge_base_id`、`is_enabled`）可加入索引 filter 字段，避免检索时回表过滤

### 6.2 检索（pipeline 新增 BM25 节点）

```sql
SELECT c.id AS chunk_id,
       c.knowledge_id,
       c.content,
       k.title AS knowledge_title,
       paradedb.score(id) AS score   -- BM25 分数
FROM chunks c
JOIN knowledges k ON c.knowledge_id = k.id AND k.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND c.metadata @@@ paradedb.match('generated_questions', :query)  -- 问题字段匹配
  AND c.knowledge_base_id = ANY(:kb_ids)
ORDER BY score DESC
LIMIT :top_k;
```

- 查询串用 `paradedb.match()` / `paradedb.query()` 构造，走 Jieba 分词后做 BM25 打分
- 关键词命中（专有名词、版本号、参数名）由 BM25 精确召回，正好补向量检索的短板

### 6.3 融合：RRF（Reciprocal Rank Fusion）

BM25 分数与向量 Cosine 分数量纲不同、不可直接相加，**只比排名不比分数**：

```
RRF(chunk) = 1 / (60 + rank_vector) + 1 / (60 + rank_bm25)
```

实现要点：

- 双路各取 TopK（如各 10 条），按 RRF 公式合并排序，截断到最终 TopK
- 接入点：`node_vector_search` 之后、`node_rerank` 之前新增 `node_hybrid_fusion` 节点，`node_rerank` 及后续节点完全不用改
- 单路无结果时降级为另一路结果，不影响主流程

### 6.4 效果示意

| 用户提问 | chunk 正文 | 纯向量匹配 | BM25 + 向量混合 |
|---|---|---|---|
| "怎么配置XXX？" | "配置步骤如下…" | 低分（语义距离远） | 问题"XXX如何配置？"BM25 精确命中 → 高召回 |

### 6.5 为什么不把问题并入向量文本

| 对比项 | 问题并入向量文本 | 问题走 BM25 稀疏索引 |
|---|---|---|
| embedding 成本 | 问题占用 token 额度 | 问题不向量化，零额外成本 |
| 关键词精确匹配 | 弱（语义向量对专有名词不敏感） | 强（BM25 精确命中术语/编号/版本号） |
| 检索链路改动 | 无需新增节点 | 需新增 BM25 检索节点 + RRF 融合节点 |
| 中文分词 | 由 embedding 模型隐式处理 | pg_search 内置 Jieba，可控可调 |

## 7. 改动清单

| 文件 | 改动 |
|---|---|
| `internal/model/entity/chunk.go` | `ChunkMetadata` 新增 `GeneratedQuestions` 字段 |
| `internal/service/knowledge_base_service.go` | `processKnowledge` 插入问题生成步骤（向量化文本保持纯正文） |
| `internal/service/knowledge_pipeline_gateway.go` | 新增 `GenerateQuestions` 接口方法（方式 B） |
| `internal/service/knowledge_pipeline_gateway_mock.go` | Mock 实现 |
| `internal/service/chat_model_factory.go`（或新文件） | 批量问题生成调用（prompt + JSON 解析 + 失败降级） |
| `internal/pipeline/node_bm25_search.go`（新增） | BM25 检索节点：`paradedb.match` + `paradedb.score` + 知识库/用户过滤 |
| `internal/pipeline/node_hybrid_fusion.go`（新增） | RRF 融合节点：合并向量与 BM25 双路结果（接入 `node_rerank` 之前） |
| `internal/pipeline/pipeline.go` | 图编排：新增 BM25 节点与融合节点 |
| `scripts/migrate.sql`（或 `ensureSchema`） | `CREATE INDEX idx_chunks_questions_bm25 ...`（Jieba 分词，幂等执行） |
| 前端（可选） | chunk 详情展示生成的问题，支持人工增删（metadata 为 JSON 天然可编辑） |

## 8. 注意点

1. **成本控制**：问题生成仅一次 LLM 批量调用（喂多个 chunk），且默认开关关闭、失败降级，风险可控；问题不参与 embedding，节省向量化 token 成本。
2. **质量问题**：LLM 生成问题可能重复/离题，prompt 中约束"只生成文档能明确回答的问题"；后续可在 chunk 详情页人工编辑 metadata 兜底。
3. **重解析一致性**：`processKnowledge` 已先删除旧 chunk 再重建，问题随 chunk 自动重建；pg_search 索引为标准 PG 索引对象，chunk 增删改自动同步，无需额外处理。
4. **pg_search 维护状态**：ParadeDB 被 Elastic 收购后独立版进入维护模式（Neon 等云厂商已停止对新项目提供），功能稳定但不再有重大更新。**建议记录当前版本号并留存安装包**，避免换环境/升级时装不上；索引创建 SQL 需幂等执行。
5. **中文分词**：默认使用 `pdb.jieba`（最准但较慢）；若索引构建耗时明显，可评估 `chinese-compatible` / `lindera` 或按问题数量控制开关。
6. **总结（可选扩展）**：若需要"上传文件总结"，建议存到 `knowledge` 表新增 `summary` 字段，与问题生成解耦，不混入 chunk 元数据。
