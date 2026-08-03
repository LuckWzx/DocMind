package docreader

import "strings"

// SelectParserEngine 根据 fileType→engine 映射匹配对应的解析引擎
func SelectParserEngine(rules map[string]string, fileType string) string {
	normalized := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
	if normalized == "" {
		return ""
	}
	if engine, ok := rules[normalized]; ok {
		return strings.TrimSpace(engine)
	}
	return ""
}

// ResolveParserEngine 解析文件类型对应的解析引擎，规则未匹配时回退到默认引擎
func ResolveParserEngine(rules map[string]string, fileType string) string {
	if parserEngine := SelectParserEngine(rules, fileType); parserEngine != "" {
		return parserEngine
	}
	return DefaultParserEngine(fileType)
}

// DefaultParserEngine 返回文件类型的默认解析引擎
func DefaultParserEngine(fileType string) string {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".") {
	case "ppt", "pptx", "csv":
		return "markitdown"
	default:
		return ""
	}
}

// DetectSourceParser 从响应元数据中检测实际使用的解析器
func DetectSourceParser(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	for _, key := range []string{"parser_engine", "parser", "engine"} {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return value
		}
	}
	return ""
}
