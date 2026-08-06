package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const dsn = "host=118.31.10.161 user=postgres password=postgres123!@# dbname=docmind port=5432 sslmode=disable"

func main() {
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Println("open err:", err)
		return
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fmt.Println("ping err:", err)
		return
	}

	attempts := []struct {
		name string
		sql  string
	}{
		{"paren pdb.jieba", `CREATE INDEX idx_tmp ON tmp_bm25_test USING bm25 (id, (content::pdb.jieba), kb_id) WITH (key_field='id')`},
		{"paren pdb.jieba 2nd", `CREATE INDEX idx_tmp ON tmp_bm25_test USING bm25 (id, (content)::pdb.jieba, kb_id) WITH (key_field='id')`},
		{"cast func pdb.jieba", `CREATE INDEX idx_tmp ON tmp_bm25_test USING bm25 (id, CAST(content AS pdb.jieba), kb_id) WITH (key_field='id')`},
		{"paren paradedb.jieba", `CREATE INDEX idx_tmp ON tmp_bm25_test USING bm25 (id, (content::paradedb.jieba), kb_id) WITH (key_field='id')`},
	}
	for _, a := range attempts {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS tmp_bm25_test"); err != nil {
			fmt.Println("drop err:", err)
			return
		}
		if _, err := db.ExecContext(ctx, `CREATE TEMP TABLE tmp_bm25_test (id bigint PRIMARY KEY, content text, kb_id bigint)`); err != nil {
			fmt.Println("create table err:", err)
			return
		}
		if _, err := db.ExecContext(ctx, a.sql); err != nil {
			fmt.Printf("[%s] err: %v\n", a.name, err)
			continue
		}
		fmt.Printf("[%s] ok\n", a.name)
		_, _ = db.ExecContext(ctx, `INSERT INTO tmp_bm25_test VALUES
			(1, '数据库连接配置教程：如何配置连接池', 7),
			(2, 'Redis 缓存配置指南', 7),
			(3, '数据库性能调优与索引优化', 8)`)
		rows, err := db.QueryContext(ctx, `SELECT id, paradedb.score(id) FROM tmp_bm25_test WHERE id @@@ paradedb.match('content', '数据库配置') ORDER BY paradedb.score(id) DESC`)
		if err != nil {
			fmt.Printf("[%s] match err: %v\n", a.name, err)
			continue
		}
		for rows.Next() {
			var id int64
			var score float64
			_ = rows.Scan(&id, &score)
			fmt.Printf("  id=%d score=%.4f\n", id, score)
		}
		rows.Close()

		// term 过滤验证
		var n int
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM tmp_bm25_test WHERE id @@@ paradedb.boolean(paradedb.match('content', '配置'), paradedb.term('kb_id', 7))`).Scan(&n)
		if err != nil {
			fmt.Printf("[%s] term filter err: %v\n", a.name, err)
		} else {
			fmt.Printf("[%s] term kb=7 count: %d\n", a.name, n)
		}
	}
}
