package tools

import (
	"fmt"
	"regexp"
	"strings"
)

// data_analysis 工具的 SQL 安全校验器。
// 借鉴 WeKnora 的 ValidateSQL 规则（只读前缀 + 单语句 + 危险函数拦截 + 表名白名单），
// 采用保守策略：宁可误杀，不可放行。

// analysisReadOnlyPrefixes 只读语句白名单（首关键字，忽略大小写）
var analysisReadOnlyPrefixes = []string{
	"select", "with", "show", "describe", "explain",
}

// analysisDangerKeywords 危险关键字黑名单（词边界匹配，忽略大小写）
var analysisDangerKeywords = []string{
	"insert", "update", "delete", "drop", "create", "alter", "replace", "truncate",
	"attach", "detach", "copy", "import", "export", "call", "install", "load",
	"set", "use", "grant", "revoke", "vacuum", "checkpoint", "prepare", "execute",
	"begin", "commit", "rollback", "pragma",
}

// analysisDangerFuncs 危险表函数/函数黑名单（包含左括号即命中）
var analysisDangerFuncs = []string{
	"read_csv(", "read_parquet(", "read_json(", "read_text(", "read_blob(",
	"read_ndjson(", "read_xlsx(", "glob(", "query_table(", "from_query(",
	"query(", "duckdb_table(", "read_objects(", "httpfs(", "postgres_scan(",
	// SQLite 特有：load_extension 可加载任意 DLL（严重 RCE 面），writefile/readfile 可读写磁盘
	"load_extension(", "writefile(", "readfile(",
}

var (
	// analysisTableRefPattern 提取 FROM/JOIN 后引用的表名（支持引号包裹与 schema 前缀）
	analysisTableRefPattern = regexp.MustCompile(`(?i)\b(?:from|join)\s+["']?([a-z0-9_."']+)["']?`)
	// analysisCTEPattern 收集 WITH 子句定义的 CTE 名（CTE 不在表名白名单内）
	analysisCTEPattern = regexp.MustCompile(`(?i)\bwith\s+([a-z0-9_]+)\s+as\s*\(`)
	// analysisDangerKeywordPatterns 危险关键字词边界匹配（预编译，忽略大小写）
	analysisDangerKeywordPatterns = func() []*regexp.Regexp {
		out := make([]*regexp.Regexp, 0, len(analysisDangerKeywords))
		for _, kw := range analysisDangerKeywords {
			out = append(out, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(kw)+`\b`))
		}
		return out
	}()
)

// stripSQLComments 去除 SQL 中的 -- 行注释与 /* */ 块注释。
// 不做字符串字面量感知（目标是拦截危险语句，极端输入宁可从严处理）。
func stripSQLComments(sql string) string {
	// 块注释
	for {
		start := strings.Index(sql, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(sql[start+2:], "*/")
		if end < 0 {
			sql = sql[:start]
			break
		}
		sql = sql[:start] + " " + sql[start+2+end+2:]
	}
	// 行注释
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// validateAnalysisSQL 校验 data_analysis 工具 SQL。
// allowedTables 为允许查询的表名集合；返回错误时调用方以降级文案返回给模型。
func validateAnalysisSQL(sql string, allowedTables []string) error {
	cleaned := strings.TrimSpace(stripSQLComments(sql))
	if cleaned == "" {
		return fmt.Errorf("SQL 为空")
	}

	// 1. 单语句：分号只能出现在语句末尾（去掉末尾分号后不得再含分号）
	trimmed := strings.TrimRight(cleaned, "; \t\r\n")
	if strings.Contains(trimmed, ";") {
		return fmt.Errorf("仅允许单条语句，禁止分号拼接多条 SQL")
	}

	// 2. 首关键字只读白名单
	lower := strings.ToLower(strings.TrimSpace(trimmed))
	okPrefix := false
	for _, p := range analysisReadOnlyPrefixes {
		if strings.HasPrefix(lower, p) {
			okPrefix = true
			break
		}
	}
	if !okPrefix {
		return fmt.Errorf("仅允许只读查询（SELECT/WITH/SHOW/DESCRIBE/EXPLAIN）")
	}

	// 3. 危险关键字黑名单（词边界）
	for i, pattern := range analysisDangerKeywordPatterns {
		if pattern.MatchString(lower) {
			return fmt.Errorf("包含禁止的关键字 %q（数据分析仅允许只读查询）", analysisDangerKeywords[i])
		}
	}

	// 4. 危险函数黑名单
	for _, fn := range analysisDangerFuncs {
		if strings.Contains(lower, fn) {
			return fmt.Errorf("包含禁止的函数 %q（禁止文件/外部访问类函数）", strings.TrimSuffix(fn, "("))
		}
	}

	// 5. 表名白名单：FROM/JOIN 引用的表必须 ∈ allowedTables 或系统表
	allow := make(map[string]struct{}, len(allowedTables))
	for _, t := range allowedTables {
		allow[strings.ToLower(t)] = struct{}{}
	}
	for _, m := range analysisCTEPattern.FindAllStringSubmatch(lower, -1) {
		allow[strings.ToLower(m[1])] = struct{}{}
	}
	for _, m := range analysisTableRefPattern.FindAllStringSubmatch(lower, -1) {
		table := strings.ToLower(strings.Trim(m[1], `"'`))
		if table == "" {
			continue
		}
		// 系统表放行（information_schema / duckdb 元数据，供 DESCRIBE 类查询）
		if strings.HasPrefix(table, "information_schema.") || strings.HasPrefix(table, "duckdb_") {
			continue
		}
		if _, ok := allow[table]; !ok {
			return fmt.Errorf("禁止查询未授权的表 %q（仅允许查询已加载的数据表）", table)
		}
	}

	return nil
}
