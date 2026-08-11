package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"docmind/internal/pipeline"

	"gorm.io/gorm"
)

// ============================================================================
// postgresKeywordDriver 基于 pg_search 扩展的 BM25 关键词检索驱动
//
// 检索链路：chunks 表建立 pg_search BM25 索引（Tantivy 引擎，jieba 中文分词），
// 倒排表由数据库自动维护（chunk 增删改自动同步），本驱动只负责：
//   1. EnsureIndex：幂等创建索引（0.22 语法：tokenizer 为列级类型转换 pdb.jieba，
//      过滤列需全部加入索引列列表，key_field 必须是主键且为第一列）
//   2. Search：构造 paradedb.match + term 组合查询，返回 BM25 分数排序的 TopK
// 用户隔离：chunks 表无 user_id 列，通过调用方传入的（已按用户过滤的）知识库列表实现。
// ============================================================================

const (
	// bm25IndexName chunks 表 BM25 索引名
	bm25IndexName = "idx_chunks_bm25"
	// bm25DefaultTopK 默认返回条数
	bm25DefaultTopK = 5
	// bm25WarmUpConns 预热查询次数：串行执行（并发预热会因索引加载锁竞争挂起，必须串行）
	bm25WarmUpConns = 3
)

// postgresKeywordDriver pg_search BM25 检索驱动
type postgresKeywordDriver struct {
	db        *gorm.DB
	indexOnce sync.Once
	warmOnce  sync.Once
	indexErr  error
}

// NewPostgresKeywordDriver 创建 pg_search BM25 检索驱动
func NewPostgresKeywordDriver(db *gorm.DB) *postgresKeywordDriver {
	return &postgresKeywordDriver{db: db}
}

// EnsureIndex 幂等创建 BM25 索引
func (d *postgresKeywordDriver) EnsureIndex(ctx context.Context) error {
	d.indexOnce.Do(func() {
		// 检查索引是否存在
		var count int
		err := d.db.WithContext(ctx).Raw(
			"SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_chunks_bm25'",
		).Scan(&count).Error
		if err != nil {
			d.indexErr = fmt.Errorf("检查索引是否存在失败: %w", err)
			return
		}
		if count > 0 {
			// 索引已存在，跳过创建
			return
		}

		// 索引不存在，执行创建
		fmt.Printf("[KeywordDriver] 创建 BM25 索引...\n")
		indexDDL := fmt.Sprintf(`
CREATE INDEX IF NOT EXISTS %s ON chunks
USING bm25 (id, (content::pdb.jieba), knowledge_base_id, is_enabled)
WITH (key_field = 'id')`, bm25IndexName)
		if err := d.db.WithContext(ctx).Exec(indexDDL).Error; err != nil {
			d.indexErr = fmt.Errorf("创建 BM25 索引失败: %w", err)
			return
		}
	})
	return d.indexErr
}

// warmUp 预热连接池。
//
// pg_search 的 Tantivy 索引状态是 per-backend（每连接）加载：新连接首次执行 @@@
// 查询需数百毫秒加载索引，之后同连接查询仅毫秒级（实测 ~500ms → ~30ms）。
// 注意：多个连接并发首次加载同一索引会因锁竞争长时间挂起（实测 5 并发 5 分钟未返回），
// 预热必须串行执行；预热后的连接留在池中，LIFO 复用使后续检索持续命中热连接。
func (d *postgresKeywordDriver) warmUp(ctx context.Context) {
	d.warmOnce.Do(func() {
		// 轻量预热查询：仅触发 Tantivy 索引加载，结果行数无关紧要
		warmSQL := "SELECT c.id FROM chunks AS c WHERE c.id @@@ paradedb.match('content', 'warmup') LIMIT 1"
		for i := 0; i < bm25WarmUpConns; i++ {
			// 预热失败只影响单个连接的加载时机，不阻塞检索主流程
			if err := d.db.WithContext(ctx).Exec(warmSQL).Error; err != nil {
				return
			}
		}
	})
}

// Search 执行 BM25 检索，返回按分数降序的 TopK 结果
func (d *postgresKeywordDriver) Search(ctx context.Context, params pipeline.PipelineKeywordSearchParams) ([]pipeline.SearchResult, error) {
	if strings.TrimSpace(params.Query) == "" || len(params.KnowledgeBaseIDs) == 0 {
		return nil, nil
	}
	if err := d.EnsureIndex(ctx); err != nil {
		return nil, err
	}
	// 预热连接池（仅首次执行）：提前加载 Tantivy 索引，避免新连接首查数百毫秒
	d.warmUp(ctx)

	topK := params.TopK
	if topK <= 0 {
		topK = bm25DefaultTopK
	}

	var results []pipeline.SearchResult
	if err := buildKeywordSearchQuery(d.db.WithContext(ctx), params, topK).Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("BM25 检索失败: %w", err)
	}

	// 阈值过滤（BM25 分数为非负任意值，不参与 SQL，内存过滤）
	if params.Threshold > 0 {
		filtered := results[:0]
		for _, r := range results {
			if r.Score >= params.Threshold {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}
	return results, nil
}

// buildKeywordSearchQuery 构造 BM25 关键词检索 SQL。
//
// 过滤条件（知识库、is_enabled）放普通 SQL WHERE：
// pg_search 会自动将其下推为 Tantivy 索引层的 must 子句，且类型编码正确（数字/布尔）。
// 不能塞进 paradedb.boolean —— 0.22 版本 boolean 是位置语义
// （第 1 个参数 must、第 2 个 should、第 3 个 must_not），且 term 参数为 text 类型，
// 对数字/布尔列过滤会失效（'true' 匹配不到 boolean true），导致过滤形同虚设。
// BM25 表达式仅保留 match 全文匹配（纯 must 子句，Tantivy 可走 top-N 快速路径）。
func buildKeywordSearchQuery(db *gorm.DB, params pipeline.PipelineKeywordSearchParams, topK int) *gorm.DB {
	return db.
		Table("chunks AS c").
		Select("c.id AS chunk_id, c.knowledge_id, c.content, k.title AS knowledge_title, paradedb.score(c.id) AS score").
		Joins("JOIN knowledges AS k ON c.knowledge_id = k.id").
		Where("c.knowledge_base_id IN ?", params.KnowledgeBaseIDs).
		Where("c.is_enabled = ?", true).
		Where("c.id @@@ paradedb.match('content', ?)", strings.TrimSpace(params.Query)).
		// 注意：Order() 仅支持 clause.OrderBy / clause.OrderByColumn / string，
		// 不能传 clause.Expr（会被静默忽略导致不排序），此处直接用字符串别名排序。
		Order("score DESC").
		Limit(topK)
}
