package longterm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"docmind/pkg/logger"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// 知识图谱提取 Prompt（参考 WeKnora extractGraphPrompt，JSON 结构约束说明更明确）
const (
	extractGraphSystemPrompt = `You are an AI assistant that extracts knowledge graphs from conversations. Extract entities, relationships and a brief summary.`

	extractGraphUserPrompt = `Given the following conversation, extract entities and relationships.

Output ONLY a valid JSON object (no markdown code block, no extra text) with this exact structure:
{
  "summary": "a brief summary of the conversation, preserving key facts, decisions and preferences",
  "entities": [
    {"title": "Entity Name", "type": "Person|Organization|Project|Product|Technology|Location|Concept|Other", "description": "short description"}
  ],
  "relationships": [
    {"source": "Entity Name (must match an entity title)", "target": "Entity Name (must match an entity title)", "description": "relationship description", "weight": 1.0}
  ]
}

Rules:
1. Entities must be specific and unambiguous full names, never pronouns
2. relationship source/target must reference entity titles extracted above
3. weight is a float between 0 and 1 indicating relationship strength
4. summary should be 2-3 sentences, in the same language as the conversation, excluding any sensitive information
5. NEVER extract or summarize sensitive information: passwords, API keys, tokens, credentials, private personal data (ID numbers, phone numbers, addresses, financial details)
6. If nothing meaningful to remember, or the conversation only contains small talk or sensitive information, return {"summary": "", "entities": [], "relationships": []}

Conversation:
%s`

	extractKeywordsSystemPrompt = `You are an AI assistant that extracts search keywords for knowledge graph retrieval.`

	extractKeywordsUserPrompt = `Extract 2-5 search keywords from the user query for searching a knowledge graph.

Output ONLY a valid JSON object (no markdown code block, no extra text) with this exact structure:
{"keywords": ["keyword1", "keyword2"]}

Rules:
1. Keywords should be entity-like terms (names, topics, project names), not generic words like "what" or "how"
2. Use the same language as the query
3. If the query has no meaningful keywords, return {"keywords": []}

Query:
%s`
)

// extractionResult LLM 提取结果（JSON 输出结构）
type extractionResult struct {
	Summary       string          `json:"summary"`
	Entities      []*Entity       `json:"entities"`
	Relationships []*Relationship `json:"relationships"`
}

type keywordsResult struct {
	Keywords []string `json:"keywords"`
}

// jsonObjectPattern 提取首个 { 到最后一个 } 之间的内容（容忍 ```json 代码块与前后杂音）
var jsonObjectPattern = regexp.MustCompile(`(?s)\{.*\}`)

// extractGraphMaxTokens 图谱提取输出上限（模型可能先输出思考/说明文字再输出 JSON，
// 2048 曾被截断导致 JSON 缺失，提高到 4096 覆盖长对话场景）
const extractGraphMaxTokens = 4096

// GraphExtractor LLM 结构化提取器：对话 → 实体/关系/摘要；query → 检索关键词。
// 模型按 modelID 懒创建 + 缓存（未触发提取时零模型开销，与短期记忆 Consolidator 同模式；
// 不同用户/会话的模型互相隔离，各缓各的实例）。
type GraphExtractor struct {
	createModel func(ctx context.Context, modelID string) (einomodel.ToolCallingChatModel, error)

	mu     sync.Mutex
	models map[string]einomodel.ToolCallingChatModel // 按 modelID 缓存模型实例
	errors map[string]error                          // 按 modelID 缓存创建失败原因（避免反复重试创建）
}

// NewGraphExtractor 创建提取器。
// createModel 为提取模型工厂，modelID 由调用方传入（当前用户会话的对话模型），
// 由调用方（app 层）负责兜底：modelID 为空/不可用时回退到默认提取模型。
func NewGraphExtractor(createModel func(ctx context.Context, modelID string) (einomodel.ToolCallingChatModel, error)) *GraphExtractor {
	return &GraphExtractor{
		createModel: createModel,
		models:      make(map[string]einomodel.ToolCallingChatModel),
		errors:      make(map[string]error),
	}
}

// ExtractGraph 从对话文本中提取摘要、实体与关系。
// modelID 为当前用户会话的对话模型（工厂内部兜底），调用失败或 JSON 解析失败时
// 重试 DefaultExtractRetries 次，仍失败返回 error（由上层降级跳过）。
func (e *GraphExtractor) ExtractGraph(ctx context.Context, modelID, conversation string) (*extractionResult, error) {
	m, err := e.getModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建提取模型失败: %w", err)
	}

	prompt := fmt.Sprintf(extractGraphUserPrompt, conversation)
	var lastErr error
	for attempt := 1; attempt <= DefaultExtractRetries+1; attempt++ {
		extractCtx, cancel := context.WithTimeout(ctx, DefaultExtractTimeout)
		resp, err := m.Generate(extractCtx, []*schema.Message{
			{Role: schema.System, Content: extractGraphSystemPrompt},
			{Role: schema.User, Content: prompt},
		}, einomodel.WithTemperature(0.1), einomodel.WithMaxTokens(extractGraphMaxTokens))
		cancel()

		if err != nil {
			lastErr = err
			logger.Warnf("[LongTermMemory] 知识图谱提取尝试 %d/%d 失败: %v", attempt, DefaultExtractRetries+1, err)
			continue
		}
		result, parseErr := parseExtractionResult(resp.Content)
		if parseErr != nil {
			lastErr = parseErr
			// 记录模型实际输出片段，便于定位是截断还是输出格式问题
			logger.Warnf("[LongTermMemory] 知识图谱提取解析尝试 %d/%d 失败: %v，模型输出前 300 字符: %q",
				attempt, DefaultExtractRetries+1, parseErr, truncateString(resp.Content, 300))
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("知识图谱提取失败 %d 次: %w", DefaultExtractRetries+1, lastErr)
}

// ExtractKeywords 从用户 query 中提取图谱检索关键词。
// modelID 为当前用户会话的对话模型（工厂内部兜底），失败时返回 error（由上层降级跳过注入，不阻断对话）。
func (e *GraphExtractor) ExtractKeywords(ctx context.Context, modelID, query string) ([]string, error) {
	m, err := e.getModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("创建提取模型失败: %w", err)
	}

	prompt := fmt.Sprintf(extractKeywordsUserPrompt, query)
	var lastErr error
	for attempt := 1; attempt <= DefaultExtractRetries+1; attempt++ {
		extractCtx, cancel := context.WithTimeout(ctx, DefaultExtractTimeout)
		resp, err := m.Generate(extractCtx, []*schema.Message{
			{Role: schema.System, Content: extractKeywordsSystemPrompt},
			{Role: schema.User, Content: prompt},
		}, einomodel.WithTemperature(0.1), einomodel.WithMaxTokens(256))
		cancel()

		if err != nil {
			lastErr = err
			logger.Warnf("[LongTermMemory] 关键词提取尝试 %d/%d 失败: %v", attempt, DefaultExtractRetries+1, err)
			continue
		}
		var result keywordsResult
		if parseErr := unmarshalJSONObject(resp.Content, &result); parseErr != nil {
			lastErr = parseErr
			logger.Warnf("[LongTermMemory] 关键词提取解析尝试 %d/%d 失败: %v", attempt, DefaultExtractRetries+1, parseErr)
			continue
		}
		// 过滤空白关键词
		keywords := result.Keywords[:0]
		for _, k := range result.Keywords {
			if k = strings.TrimSpace(k); k != "" {
				keywords = append(keywords, k)
			}
		}
		return keywords, nil
	}
	return nil, fmt.Errorf("关键词提取失败 %d 次: %w", DefaultExtractRetries+1, lastErr)
}

// getModel 按 modelID 懒创建 + 缓存提取模型（并发安全）
func (e *GraphExtractor) getModel(ctx context.Context, modelID string) (einomodel.ToolCallingChatModel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if m := e.models[modelID]; m != nil {
		return m, nil
	}
	if err := e.errors[modelID]; err != nil {
		return nil, err
	}
	m, err := e.createModel(ctx, modelID)
	if err != nil {
		e.errors[modelID] = err
		return nil, err
	}
	e.models[modelID] = m
	return m, nil
}

// parseExtractionResult 解析 LLM 输出的图谱 JSON，并做基础校验
func parseExtractionResult(content string) (*extractionResult, error) {
	var result extractionResult
	if err := unmarshalJSONObject(content, &result); err != nil {
		return nil, err
	}
	// 规范化：剔除空白实体名/关系端点，避免落图脏数据
	entities := result.Entities[:0]
	for _, en := range result.Entities {
		if en == nil || strings.TrimSpace(en.Title) == "" {
			continue
		}
		en.Title = strings.TrimSpace(en.Title)
		entities = append(entities, en)
	}
	result.Entities = entities

	relations := result.Relationships[:0]
	for _, rel := range result.Relationships {
		if rel == nil || strings.TrimSpace(rel.Source) == "" || strings.TrimSpace(rel.Target) == "" {
			continue
		}
		rel.Source = strings.TrimSpace(rel.Source)
		rel.Target = strings.TrimSpace(rel.Target)
		if rel.Weight < 0 {
			rel.Weight = 0
		}
		if rel.Weight > 1 {
			rel.Weight = 1
		}
		relations = append(relations, rel)
	}
	result.Relationships = relations
	return &result, nil
}

// truncateString 截断字符串到指定长度（日志输出用，避免刷屏）
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// unmarshalJSONObject 从 LLM 输出中提取 JSON 对象并解析（容忍代码块包裹）
func unmarshalJSONObject(content string, v any) error {
	match := jsonObjectPattern.FindString(content)
	if match == "" {
		return fmt.Errorf("LLM 输出中未找到 JSON 对象")
	}
	if err := json.Unmarshal([]byte(match), v); err != nil {
		return fmt.Errorf("JSON 解析失败: %w", err)
	}
	return nil
}
