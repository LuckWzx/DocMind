package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"docmind/internal/pipeline"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
)

// postgresKeywordDriver pg_search BM25 检索驱动
type postgresKeywordDriver struct {
	db *gorm.DB
}

// NewPostgresKeywordDriver 创建 pg_search BM25 检索驱动
func NewPostgresKeywordDriver(db *gorm.DB) *postgresKeywordDriver {
	return &postgresKeywordDriver{db: db}
}

// EnsureIndex 幂等创建 BM25 索引
func (d *postgresKeywordDriver) EnsureIndex(ctx context.Context) error {
	if err := d.db.WithContext(ctx).Exec("CREATE EXTENSION IF NOT EXISTS pg_search").Error; err != nil {
		return fmt.Errorf("启用 pg_search 扩展失败: %w", err)
	}
	indexDDL := fmt.Sprintf(`
CREATE INDEX IF NOT EXISTS %s ON chunks
USING bm25 (id, (content::pdb.jieba), knowledge_base_id, is_enabled)
WITH (key_field = 'id')`, bm25IndexName)
	if err := d.db.WithContext(ctx).Exec(indexDDL).Error; err != nil {
		return fmt.Errorf("创建 BM25 索引失败: %w", err)
	}
	return nil
}

// Search 执行 BM25 检索，返回按分数降序的 TopK 结果
func (d *postgresKeywordDriver) Search(ctx context.Context, params pipeline.PipelineKeywordSearchParams) ([]pipeline.SearchResult, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" || len(params.KnowledgeBaseIDs) == 0 {
		return nil, nil
	}
	if err := d.EnsureIndex(ctx); err != nil {
		return nil, err
	}

	topK := params.TopK
	if topK <= 0 {
		topK = bm25DefaultTopK
	}

	searchExpr := buildBM25SearchExpr(query, params.KnowledgeBaseIDs)

	var results []pipeline.SearchResult
	err := d.db.WithContext(ctx).
		Table("chunks AS c").
		Select("c.id AS chunk_id, c.knowledge_id, c.content, k.title AS knowledge_title, paradedb.score(c.id) AS score").
		Joins("JOIN knowledges AS k ON c.knowledge_id = k.id AND k.deleted_at IS NULL").
		Where("c.deleted_at IS NULL AND c.is_enabled = ?", true).
		Where(searchExpr).
		// 注意：Order() 仅支持 clause.OrderBy / clause.OrderByColumn / string，
		// 不能传 clause.Expr（会被静默忽略导致不排序），此处直接用字符串别名排序。
		Order("score DESC").
		Limit(topK).
		Scan(&results).Error
	if err != nil {
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

// buildBM25SearchExpr 构造 pg_search 检索表达式（match 全文匹配 + 知识库 term 过滤）
//   - 单知识库：boolean(match(content, ?), term(knowledge_base_id, ?))
//   - 多知识库：boolean(match(content, ?), boolean(term(kb, ?), term(kb, ?), occurrence => 'or'))
//
// 注意：term 的 value 参数在 pg_search 中为 text 类型，需传字符串（pgx 严格类型编码）
func buildBM25SearchExpr(query string, kbIDs []uint) clause.Expr {
	vars := []interface{}{query}
	var filterSQL string
	if len(kbIDs) == 1 {
		vars = append(vars, strconv.FormatUint(uint64(kbIDs[0]), 10))
		filterSQL = "paradedb.term('knowledge_base_id', ?)"
	} else {
		terms := make([]string, 0, len(kbIDs))
		for _, id := range kbIDs {
			terms = append(terms, "paradedb.term('knowledge_base_id', ?)")
			vars = append(vars, strconv.FormatUint(uint64(id), 10))
		}
		filterSQL = "paradedb.boolean(" + strings.Join(terms, ", ") + ", occurrence => 'or')"
	}
	return clause.Expr{
		SQL:  "c.id @@@ paradedb.boolean(paradedb.match('content', ?), " + filterSQL + ")",
		Vars: vars,
	}
}
