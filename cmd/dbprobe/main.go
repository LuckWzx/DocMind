// dbprobe 临时诊断工具：直连远程 PostgreSQL，实测 KeywordSearch 相关查询耗时与执行计划
package main

import (
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const dsn = "host=118.31.10.161 user=postgres password=postgres123!@# dbname=docmind port=5432 sslmode=disable TimeZone=Asia/Shanghai"

func main() {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("连接失败:", err)
		os.Exit(1)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	// 单条语句超时保护
	db.Exec("SET statement_timeout = '60s'")

	queries := []struct {
		name string
		sql  string
	}{
		{"pg_search 版本", "SELECT extversion FROM pg_extension WHERE extname='pg_search'"},
		{"BM25 索引定义", "SELECT indexdef FROM pg_indexes WHERE indexname='idx_chunks_bm25'"},
		{"chunks 总量", "SELECT count(*) AS total FROM chunks"},
		{"各知识库 chunk 分布", "SELECT knowledge_base_id, count(*) AS total, count(*) FILTER (WHERE NOT is_enabled) AS disabled FROM chunks GROUP BY 1 ORDER BY 2 DESC LIMIT 15"},
		{"A. 仅 term 过滤(无 match)", "SELECT c.id FROM chunks c WHERE c.id @@@ paradedb.boolean(paradedb.term('knowledge_base_id', '6'), paradedb.term('is_enabled', 'true')) LIMIT 5"},
		{"B. 仅 match(无 kb 过滤)", "SELECT c.id FROM chunks c WHERE c.id @@@ paradedb.match('content', '请举例说明我可以询问的问题。') LIMIT 5"},
		{"C. 完整查询(修复后参数顺序)", "SELECT c.id AS chunk_id, c.knowledge_id, c.content, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.id @@@ paradedb.boolean(paradedb.match('content', '请举例说明我可以询问的问题。'), paradedb.term('knowledge_base_id', '6'), paradedb.term('is_enabled', 'true')) ORDER BY score DESC LIMIT 5"},
		{"D. 完整查询 EXPLAIN", "EXPLAIN (ANALYZE, BUFFERS) SELECT c.id AS chunk_id, c.knowledge_id, c.content, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.id @@@ paradedb.boolean(paradedb.match('content', '请举例说明我可以询问的问题。'), paradedb.term('knowledge_base_id', '6'), paradedb.term('is_enabled', 'true')) ORDER BY score DESC LIMIT 5"},
		{"E. 位置语义验证 boolean(term6, term68)", "SELECT c.id, c.knowledge_base_id FROM chunks c WHERE c.id @@@ paradedb.boolean(paradedb.term('knowledge_base_id', '6'), paradedb.term('knowledge_base_id', '68')) LIMIT 10"},
		{"F. occurrence=>'must' 支持验证", "SELECT c.id, c.knowledge_base_id FROM chunks c WHERE c.id @@@ paradedb.boolean(paradedb.term('knowledge_base_id', '6'), paradedb.term('knowledge_base_id', '68'), occurrence => 'must') LIMIT 10"},
		{"G. term is_enabled 'true' 验证", "SELECT c.id, c.is_enabled FROM chunks c WHERE c.id @@@ paradedb.term('is_enabled', 'true') LIMIT 5"},
		{"H. 完整查询 occurrence=>'must' 耗时", "SELECT c.id AS chunk_id, c.knowledge_id, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.id @@@ paradedb.boolean(paradedb.match('content', '请举例说明我可以询问的问题。'), paradedb.term('knowledge_base_id', '6'), paradedb.term('is_enabled', 'true'), occurrence => 'must') ORDER BY score DESC LIMIT 5"},
		{"I. 完整查询 occurrence=>'must' EXPLAIN", "EXPLAIN (ANALYZE, BUFFERS) SELECT c.id AS chunk_id, c.knowledge_id, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.id @@@ paradedb.boolean(paradedb.match('content', '请举例说明我可以询问的问题。'), paradedb.term('knowledge_base_id', '6'), paradedb.term('is_enabled', 'true'), occurrence => 'must') ORDER BY score DESC LIMIT 5"},
		{"K. SQL WHERE 过滤 + 纯 match 耗时", "SELECT c.id AS chunk_id, c.knowledge_id, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.knowledge_base_id = 6 AND c.is_enabled = true AND c.id @@@ paradedb.match('content', '请举例说明我可以询问的问题。') ORDER BY score DESC LIMIT 5"},
		{"L. SQL WHERE 过滤 + 纯 match EXPLAIN", "EXPLAIN (ANALYZE, BUFFERS) SELECT c.id AS chunk_id, c.knowledge_id, k.title AS knowledge_title, paradedb.score(c.id) AS score FROM chunks AS c JOIN knowledges AS k ON c.knowledge_id = k.id WHERE c.knowledge_base_id = 6 AND c.is_enabled = true AND c.id @@@ paradedb.match('content', '请举例说明我可以询问的问题。') ORDER BY score DESC LIMIT 5"},
	}

	for _, q := range queries {
		runQuery(db, q.name, q.sql)
	}
}

func runQuery(db *gorm.DB, name, sql string) {
	start := time.Now()
	rows, err := db.Raw(sql).Rows()
	if err != nil {
		fmt.Printf("[%s] 错误: %v\n", name, err)
		return
	}
	cols, _ := rows.Columns()
	var results []string
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		line := ""
		for i, v := range vals {
			b, _ := v.([]byte)
			if i > 0 {
				line += " | "
			}
			if b != nil {
				line += string(b)
			} else {
				line += fmt.Sprintf("%v", v)
			}
		}
		results = append(results, line)
	}
	rows.Close()
	elapsed := time.Since(start)
	fmt.Printf("\n===== %s [%v] =====\n", name, elapsed)
	for _, r := range results {
		fmt.Println(r)
	}
}
