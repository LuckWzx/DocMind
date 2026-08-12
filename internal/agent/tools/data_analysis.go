package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"docmind/internal/model/entity"
	"docmind/internal/repository"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（无 cgo 依赖，CGO_ENABLED=0 可编译）
)

// analysisSession 单次 Agent 请求的数据分析会话：共享一个 DuckDB 内存库。
// 生命周期 = 一次 AgentChat 请求（Registry.Build 创建 → collector.Cleanup 关闭），
// 请求内 data_analysis / data_schema 工具共享已加载的表（模型可先看结构再写 SQL），
// 跨请求自动隔离（每次请求都是全新内存库），无需持久化清理。
type analysisSession struct {
	mu     sync.Mutex
	db     *sql.DB
	loaded map[string]*analysisTableInfo // 表名 → 结构信息（幂等加载）
}

// analysisColumn 单列信息
type analysisColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable"`
}

// analysisTableInfo 已加载表的结构信息
type analysisTableInfo struct {
	TableName string           `json:"table_name"`
	Columns   []analysisColumn `json:"columns"`
	RowCount  int64            `json:"row_count"`
}

// excelSheetNameColumn 多 sheet Excel 合并后标记来源 sheet 的合成列名
const excelSheetNameColumn = "__sheet_name"

// newAnalysisSession 创建数据分析会话（SQLite 内存库，纯 Go 驱动，无 cgo 依赖）。
// 内存库必须限制为单连接：database/sql 连接池的每个连接各自持有独立内存库，
// 多连接会导致建的表在另一连接上不可见。
func newAnalysisSession() (*analysisSession, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("初始化分析数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &analysisSession{db: db, loaded: map[string]*analysisTableInfo{}}, nil
}

// Close 关闭会话（幂等）
func (s *analysisSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
}

// loadTable 加载文件到 DuckDB 表（幂等：已加载直接返回结构信息）
func (s *analysisSession) loadTable(ctx context.Context, filePath, fileType, tableName string) (*analysisTableInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, ok := s.loaded[tableName]; ok {
		return info, nil
	}

	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "csv":
		if err := s.loadCSV(ctx, filePath, tableName); err != nil {
			return nil, err
		}
	case "xlsx":
		if err := s.loadXLSX(ctx, filePath, tableName); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("不支持的文件类型 %q（仅支持 csv/xlsx）", fileType)
	}

	info, err := s.describeTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	s.loaded[tableName] = info
	return info, nil
}

// loadCSV 解析 CSV（编码自动识别 UTF-8/GBK/UTF-16）→ 类型推断建表 → 批量插入。
// 不使用数据库驱动的文件表函数（read_csv_auto 等），保证 CGO_ENABLED=0 环境下可编译。
func (s *analysisSession) loadCSV(ctx context.Context, filePath, tableName string) error {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取 CSV 文件失败: %w", err)
	}
	decoded, err := decodeTabularBytes(raw)
	if err != nil {
		return err
	}

	reader := csv.NewReader(bytes.NewReader(decoded))
	reader.FieldsPerRecord = -1 // 行字段数不一致时不报错（缺失列按空处理）
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("解析 CSV 失败: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("CSV 文件为空")
	}
	return s.createTableFromRows(ctx, tableName, records[0], records[1:])
}

// loadXLSX 用 excelize 解析 Excel（多 sheet 合并为一张表，追加 __sheet_name 合成列）
// → 类型推断建表 → 批量插入。
// 多 sheet 列结构差异处理：列名取全部 sheet 的并集（保序），缺失列补空。
func (s *analysisSession) loadXLSX(ctx context.Context, filePath, tableName string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("解析 Excel 文件失败: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("Excel 文件没有可读取的工作表")
	}

	// 1. 读取全部 sheet 的行数据 + 收集列名并集
	type sheetRows struct {
		name string
		rows [][]string
	}
	var all []sheetRows
	colOrder := make([]string, 0, 32)
	colSeen := make(map[string]struct{}, 32)
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return fmt.Errorf("读取工作表 %q 失败: %w", sheet, err)
		}
		all = append(all, sheetRows{name: sheet, rows: rows})
		if len(rows) > 0 {
			for _, c := range rows[0] {
				if _, ok := colSeen[c]; !ok {
					colSeen[c] = struct{}{}
					colOrder = append(colOrder, c)
				}
			}
		}
	}
	if len(colOrder) == 0 {
		return fmt.Errorf("Excel 文件没有表头数据")
	}

	// 2. 按合并列名对齐各 sheet 数据行（缺失列补空），末尾追加 __sheet_name
	mergedRows := make([][]string, 0, 4096)
	for _, sr := range all {
		for i, row := range sr.rows {
			if i == 0 {
				continue // 跳过表头行
			}
			values := make([]string, 0, len(colOrder)+1)
			cellMap := make(map[string]string, len(colOrder))
			for j, cell := range row {
				if j < len(colOrder) {
					cellMap[colOrder[j]] = cell
				}
			}
			for _, c := range colOrder {
				values = append(values, cellMap[c])
			}
			values = append(values, sr.name)
			mergedRows = append(mergedRows, values)
		}
	}
	header := append(append([]string{}, colOrder...), excelSheetNameColumn)
	return s.createTableFromRows(ctx, tableName, header, mergedRows)
}

// decodeTabularBytes 规范化表格文本编码：UTF-8（含 BOM）直通，UTF-16/GBK 转 UTF-8。
func decodeTabularBytes(raw []byte) ([]byte, error) {
	// UTF-8 BOM
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return raw[3:], nil
	}
	// UTF-16 LE/BE BOM
	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) {
		decoded, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(), raw)
		return decoded, err
	}
	if bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		decoded, _, err := transform.Bytes(unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder(), raw)
		return decoded, err
	}
	if utf8.Valid(raw) {
		return raw, nil
	}
	// 尝试 GBK（常见于国内 CSV 导出）
	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), raw)
	if err != nil {
		return nil, fmt.Errorf("文件编码无法识别（仅支持 UTF-8 / GBK / UTF-16）: %w", err)
	}
	return decoded, nil
}

// createTableFromRows 列名清洗 + 类型推断 + 建表 + 事务批量插入（CSV 与 Excel 共用入口）
func (s *analysisSession) createTableFromRows(ctx context.Context, tableName string, header []string, rows [][]string) error {
	// 1. 列名清洗：去空白、空列名补默认名、重名加后缀；列类型先默认 TEXT
	cols := make([]string, len(header))
	colTypes := make([]string, len(header))
	seen := make(map[string]int, len(header))
	for i, h := range header {
		name := strings.TrimSpace(h)
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		if n, ok := seen[name]; ok {
			seen[name] = n + 1
			name = fmt.Sprintf("%s_%d", name, n+1)
		} else {
			seen[name] = 1
		}
		cols[i] = name
		colTypes[i] = "TEXT"
	}

	// 2. 类型推断（按列采样：全部可解析为整数 → INTEGER；全部可解析为浮点 → REAL）
	for i := range colTypes {
		colTypes[i] = inferColumnType(rows, i)
	}

	// 3. 建表
	colDefs := make([]string, len(cols))
	for i, c := range cols {
		colDefs[i] = fmt.Sprintf("%s %s", escapeSQLIdent(c), colTypes[i])
	}
	createSQL := fmt.Sprintf(`CREATE TABLE %s (%s)`, escapeSQLIdent(tableName), strings.Join(colDefs, ", "))
	if _, err := s.db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("创建数据表失败: %w", err)
	}

	// 4. 事务批量插入（空值写 NULL，数字列可空）
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启写入事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf(`INSERT INTO %s VALUES (%s)`, escapeSQLIdent(tableName), strings.Join(placeholders, ", "))
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("准备写入语句失败: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, row := range rows {
		args := make([]interface{}, len(cols))
		for i := range cols {
			if i < len(row) && strings.TrimSpace(row[i]) != "" {
				args[i] = row[i]
			} else {
				args[i] = nil
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("写入数据行失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交写入事务失败: %w", err)
	}
	return nil
}

// inferColumnType 推断列类型：全部非空值可解析为整数 → INTEGER；可解析为浮点 → REAL；否则 TEXT
func inferColumnType(rows [][]string, colIdx int) string {
	hasValue := false
	allInt, allReal := true, true
	for _, row := range rows {
		if colIdx >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[colIdx])
		if v == "" {
			continue
		}
		hasValue = true
		if _, err := strconv.ParseInt(v, 10, 64); err != nil {
			allInt = false
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			allReal = false
		}
		if !allInt && !allReal {
			return "TEXT"
		}
	}
	if !hasValue {
		return "TEXT"
	}
	if allInt {
		return "INTEGER"
	}
	if allReal {
		return "REAL"
	}
	return "TEXT"
}

// escapeSQLIdent 转义 SQL 标识符（双引号内双引号）
func escapeSQLIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// describeTable 查询表结构与行数（PRAGMA table_info）
func (s *analysisSession) describeTable(ctx context.Context, tableName string) (*analysisTableInfo, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, escapeSQLIdent(tableName)))
	if err != nil {
		return nil, fmt.Errorf("获取表结构失败: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols := make([]analysisColumn, 0, 16)
	for rows.Next() {
		var cid int
		var c analysisColumn
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("解析表结构失败: %w", err)
		}
		c.Nullable = "YES"
		if notNull == 1 {
			c.Nullable = "NO"
		}
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历表结构失败: %w", err)
	}

	var rowCount int64
	if err := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, escapeSQLIdent(tableName))).Scan(&rowCount); err != nil {
		return nil, fmt.Errorf("统计行数失败: %w", err)
	}
	return &analysisTableInfo{TableName: tableName, Columns: cols, RowCount: rowCount}, nil
}

// executeQuery 执行查询并返回行结果（键为列名，值为字符串）
func (s *analysisSession) executeQuery(ctx context.Context, query string) ([]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	results := make([]map[string]string, 0, 64)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]string, len(columns))
		for i, col := range columns {
			if b, ok := values[i].([]byte); ok {
				rowMap[col] = string(b)
			} else if values[i] == nil {
				rowMap[col] = ""
			} else {
				rowMap[col] = fmt.Sprintf("%v", values[i])
			}
		}
		results = append(results, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// formatAnalysisResults 查询结果 → 模型可读文本（JSONL 格式，行数提示）
func formatAnalysisResults(results []map[string]string, query string, info *analysisTableInfo) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== 查询结果 ===\n执行 SQL: %s\n返回 %d 行\n\n", query, len(results)))
	if len(results) == 0 {
		b.WriteString("没有匹配的记录。\n")
		return b.String()
	}
	if len(results) > 10 {
		b.WriteString(fmt.Sprintf("共 %d 条记录，建议在 SQL 中加 LIMIT 限制返回条数。\n\n", len(results)))
	}
	for i, record := range results {
		recordBytes, _ := json.Marshal(record)
		b.WriteString(fmt.Sprintf("record %d: %s\n", i+1, recordBytes))
	}
	return b.String()
}

// formatAnalysisSchema 表结构 → 模型可读文本（data_analysis 加载时与 data_schema 共用）
func formatAnalysisSchema(info *analysisTableInfo) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("表 %s 已加载：%d 列，%d 行\n", info.TableName, len(info.Columns), info.RowCount))
	b.WriteString("列结构：\n")
	for _, c := range info.Columns {
		b.WriteString(fmt.Sprintf("- %s (%s)\n", c.Name, c.Type))
	}
	return b.String()
}

// resolveAnalysisKnowledge 校验并解析知识条目：归属当前用户 + 在 Agent 知识库范围内。
// 返回知识条目与数据表名（k_<id>）。错误文案统一为"不存在或无权访问"，避免越权探测。
func resolveAnalysisKnowledge(
	ctx context.Context,
	knowledgeRepo repository.KnowledgeRepository,
	kbRepo repository.KnowledgeBaseRepository,
	userID uint,
	kbScope []string,
	knowledgeIDStr string,
) (*entity.Knowledge, string, error) {
	kid := parseUintID(knowledgeIDStr)
	if kid == 0 {
		return nil, "", fmt.Errorf("无效的 knowledge_id")
	}
	knowledge, err := knowledgeRepo.FindByID(kid)
	if err != nil || knowledge == nil {
		return nil, "", fmt.Errorf("知识条目不存在或无权访问")
	}
	// 归属校验：知识库必须属于当前用户
	kb, err := kbRepo.FindByUserID(userID, knowledge.KnowledgeBaseID)
	if err != nil || kb == nil {
		return nil, "", fmt.Errorf("知识条目不存在或无权访问")
	}
	// 范围校验：Agent 指定了知识库时，条目必须在其范围内
	if len(kbScope) > 0 {
		inScope := false
		for _, id := range kbScope {
			if id == fmt.Sprintf("%d", knowledge.KnowledgeBaseID) {
				inScope = true
				break
			}
		}
		if !inScope {
			return nil, "", fmt.Errorf("知识条目不在当前智能体的知识库范围内")
		}
	}
	return knowledge, fmt.Sprintf("k_%d", knowledge.ID), nil
}

// ===== data_analysis 工具 =====

// DataAnalysisArgs data_analysis 工具参数（模型按 JSON Schema 填充）
type DataAnalysisArgs struct {
	KnowledgeID string `json:"knowledge_id"`
	SQL         string `json:"sql"`
}

// NewDataAnalysisTool 构建数据分析工具：
// 把知识库中的 CSV/Excel 加载进 DuckDB 内存表，执行只读 SQL 做统计计算。
// 闭包捕获：知识仓储（归属校验）、DuckDB 会话（请求级共享）、用户上下文（知识库范围）。
func NewDataAnalysisTool(
	knowledgeRepo repository.KnowledgeRepository,
	kbRepo repository.KnowledgeBaseRepository,
	userID uint,
	kbScope []string,
	session *analysisSession,
) (tool.BaseTool, error) {
	analysisFn := func(ctx context.Context, args DataAnalysisArgs) (string, error) {
		// 1. 归属校验 + 元数据
		knowledge, tableName, err := resolveAnalysisKnowledge(ctx, knowledgeRepo, kbRepo, userID, kbScope, args.KnowledgeID)
		if err != nil {
			return fmt.Sprintf("数据分析失败：%v。请确认 knowledge_id 为知识库中 CSV/Excel 文件的条目 ID。", err), nil
		}
		// 远程文件（URL 存储）暂不支持：DuckDB/文件系统无法直接读取
		if isAnalysisRemoteURL(knowledge.FileURL) {
			return "数据分析失败：远程存储的文件暂不支持数据分析，请使用本地存储的知识库。", nil
		}

		// 2. 加载数据（幂等：同一会话内重复查询不重复加载）
		info, err := session.loadTable(ctx, knowledge.FileURL, knowledge.FileType, tableName)
		if err != nil {
			return fmt.Sprintf("数据分析失败：%v。请确认文件为 UTF-8/GBK 编码的 CSV 或 .xlsx 格式。", err), nil
		}

		// 3. SQL 安全校验（只读 + 单语句 + 表名白名单）
		if err := validateAnalysisSQL(args.SQL, []string{tableName}); err != nil {
			return fmt.Sprintf("SQL 校验未通过：%v。请只使用只读查询（SELECT/WITH/SHOW/DESCRIBE/EXPLAIN），且只能查询表 %s。\n%s",
				err, tableName, formatAnalysisSchema(info)), nil
		}

		// 4. 执行查询
		results, err := session.executeQuery(ctx, args.SQL)
		if err != nil {
			return fmt.Sprintf("查询执行失败：%v。请检查 SQL 语法与列名（可用 SHOW 查看列结构）。\n%s", err, formatAnalysisSchema(info)), nil
		}
		return formatAnalysisResults(results, args.SQL, info), nil
	}

	return utils.InferTool[DataAnalysisArgs, string](
		"data_analysis",
		"对知识库中的表格文件（CSV/Excel）执行数据分析。当用户问题需要对表格数据做统计计算（求和、平均、分组、排序、筛选等）时必须调用本工具。\n"+
			"## 使用方式\n"+
			"1. 参数 knowledge_id 为表格文件对应知识条目的 ID（先调用 kb_search 检索该表格内容，从检索结果的 knowledge_id 字段获取）；\n"+
			"2. 参数 sql 为要执行的 SQL（仅支持 SELECT/WITH/SHOW/DESCRIBE/EXPLAIN 只读语句）；\n"+
			"3. 文件首次被引用时自动加载为数据表（表名 k_<id>），同一轮对话内重复查询不会重复加载；\n"+
			"4. Excel 多工作表会合并为一张表，来源工作表名保存在 __sheet_name 列，可按该列过滤；\n"+
			"5. 查询前可先用 SHOW 语句查看表结构与列名。\n"+
			"## 注意事项\n"+
			"- 只能查询本次加载的数据表，禁止跨表/跨文件引用；\n"+
			"- 数据行数较多时请使用 LIMIT 限制返回条数，避免结果过长。",
		analysisFn,
	)
}

// isAnalysisRemoteURL 判断文件定位是否为远程 URL（数据分析暂不支持）
func isAnalysisRemoteURL(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
