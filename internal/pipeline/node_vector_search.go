package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"docmind/internal/model/entity"
)

// newVectorSearchNode 创建向量检索节点（闭包注入外部依赖）
func newVectorSearchNode(deps *PipelineDeps) func(ctx context.Context, input *Context) (*Context, error) {
	return func(ctx context.Context, input *Context) (*Context, error) {
		startTime := time.Now()
		toolCallID := "vector_search"

		// 发送步骤开始回调
		if input.StepCallback != nil {
			input.StepCallback(StepInfo{
				StepName:   "vector_search",
				StartTime:  startTime,
				ToolCallID: toolCallID,
				Success:    true,
			})
		}

		// 如果没有知识库，跳过检索
		if len(input.AgentConfig.KnowledgeBaseIDs) == 0 {
			input.SearchResults = []SearchResult{}
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    true,
					Data:       map[string]interface{}{"count": 0},
				})
			}

			return input, nil
		}

		// 依赖未注入时降级为空结果
		if deps == nil || deps.EmbedderFactory == nil || deps.VectorStoreRepo == nil {
			input.SearchResults = []SearchResult{}
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    true,
					Data:       map[string]interface{}{"count": 0},
				})
			}

			return input, nil
		}

		// 1. 查询改写后的文本作为检索 query
		query := input.RewrittenQuery
		if query == "" {
			query = input.Query
		}

		// 2. 根据第一个知识库确定 Embedding 模型
		kbID := parseUint(input.AgentConfig.KnowledgeBaseIDs[0])
		kb, err := deps.KBRepo.FindByID(kbID)
		if err != nil || kb == nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("知识库不存在: %s", input.AgentConfig.KnowledgeBaseIDs[0])},
				})
			}

			return nil, fmt.Errorf("向量检索: 知识库不存在: %s", input.AgentConfig.KnowledgeBaseIDs[0])
		}

		embedderID := kb.EmbeddingModelID
		if embedderID == "" {
			embedderID = "default"
		}

		// 3. 创建 Embedder 并生成查询向量
		embedder, err := deps.EmbedderFactory.CreateEmbedder(ctx, embedderID)
		if err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("创建 Embedder 失败: %v", err)},
				})
			}

			return nil, fmt.Errorf("向量检索: 创建 Embedder 失败: %w", err)
		}

		// EmbedStrings 返回单条文本的向量 []float64
		vectors, err := embedder.EmbedStrings(ctx, []string{query})
		if err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("Embedding 调用失败: %v", err)},
				})
			}

			return nil, fmt.Errorf("向量检索: Embedding 调用失败: %w", err)
		}
		if len(vectors) == 0 {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": "Embedding 返回空结果"},
				})
			}

			return nil, fmt.Errorf("向量检索: Embedding 返回空结果")
		}

		// 将 []float64 转为 []float32（pgvector 使用 float32）
		queryVector := make([]float32, len(vectors))
		for i, v := range vectors {
			queryVector[i] = float32(v)
		}

		// 4. 查找向量存储：优先知识库配置的向量存储，没有则回退到系统全局默认存储
		//    知识库的 VectorStoreID 是权威配置，不做用户可见性过滤，与写入侧 persistVectors 保持一致
		var store *entity.VectorStore
		if kb.VectorStoreID != nil && *kb.VectorStoreID > 0 {
			store, err = deps.VectorStoreRepo.FindByID(*kb.VectorStoreID)
			if err != nil {
				endTime := time.Now()
				duration := endTime.Sub(startTime).Milliseconds()

				// 发送步骤完成回调
				if input.StepCallback != nil {
					input.StepCallback(StepInfo{
						StepName:   "vector_search",
						StartTime:  startTime,
						EndTime:    endTime,
						Duration:   duration,
						ToolCallID: toolCallID,
						Success:    false,
						Data:       map[string]interface{}{"error": fmt.Sprintf("查询知识库向量存储失败: %v", err)},
					})
				}

				return nil, fmt.Errorf("向量检索: 查询知识库向量存储失败: %w", err)
			}
		}
		if store == nil {
			store, err = deps.VectorStoreRepo.FirstOrCreateGlobalDefault()
			if err != nil {
				endTime := time.Now()
				duration := endTime.Sub(startTime).Milliseconds()

				// 发送步骤完成回调
				if input.StepCallback != nil {
					input.StepCallback(StepInfo{
						StepName:   "vector_search",
						StartTime:  startTime,
						EndTime:    endTime,
						Duration:   duration,
						ToolCallID: toolCallID,
						Success:    false,
						Data:       map[string]interface{}{"error": fmt.Sprintf("获取默认向量存储失败: %v", err)},
					})
				}

				return nil, fmt.Errorf("向量检索: 获取默认向量存储失败: %w", err)
			}
		}

		// 5. 创建向量驱动
		driver, cleanup, err := deps.CreateDriver(store)
		if err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("创建向量驱动失败: %v", err)},
				})
			}

			return nil, fmt.Errorf("向量检索: 创建向量驱动失败: %w", err)
		}
		defer cleanup()

		if err := driver.EnsureSchema(ctx); err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("初始化向量表失败: %v", err)},
				})
			}

			return nil, fmt.Errorf("向量检索: 初始化向量表失败: %w", err)
		}

		// 6. 转换知识库 ID 并执行检索
		uintKBIDs := make([]uint, 0, len(input.AgentConfig.KnowledgeBaseIDs))
		for _, id := range input.AgentConfig.KnowledgeBaseIDs {
			uintKBIDs = append(uintKBIDs, parseUint(id))
		}

		topK := input.AgentConfig.EmbeddingTopK
		if topK <= 0 {
			topK = 5
		}
		threshold := input.AgentConfig.VectorThreshold

		results, err := driver.Search(ctx, PipelineVectorSearchParams{
			UserID:           input.UserID,
			VectorStoreID:    store.ID,
			KnowledgeBaseIDs: uintKBIDs,
			QueryVector:      queryVector,
			TopK:             topK,
			Threshold:        threshold,
		})
		if err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime).Milliseconds()

			// 发送步骤完成回调
			if input.StepCallback != nil {
				input.StepCallback(StepInfo{
					StepName:   "vector_search",
					StartTime:  startTime,
					EndTime:    endTime,
					Duration:   duration,
					ToolCallID: toolCallID,
					Success:    false,
					Data:       map[string]interface{}{"error": fmt.Sprintf("检索失败: %v", err)},
				})
			}

			return nil, fmt.Errorf("向量检索: 检索失败: %w", err)
		}

		// 7. 转换结果
		searchResults := make([]SearchResult, 0, len(results))
		for _, r := range results {
			searchResults = append(searchResults, SearchResult{
				ChunkID:        r.ChunkID,
				Score:          r.Score,
				KnowledgeID:    r.KnowledgeID,
				Content:        r.Content,
				KnowledgeTitle: r.KnowledgeTitle,
			})
		}

		input.SearchResults = searchResults
		endTime := time.Now()
		duration := endTime.Sub(startTime).Milliseconds()

		// 发送步骤完成回调
		if input.StepCallback != nil {
			input.StepCallback(StepInfo{
				StepName:   "vector_search",
				StartTime:  startTime,
				EndTime:    endTime,
				Duration:   duration,
				ToolCallID: toolCallID,
				Success:    true,
				Data:       map[string]interface{}{"count": len(searchResults)},
			})
		}

		fmt.Printf("[Pipeline] VectorSearch: query=%s, kbIDs=%v, results=%d\n",
			query, input.AgentConfig.KnowledgeBaseIDs, len(searchResults))

		return input, nil
	}
}

// parseUint 解析字符串为 uint
func parseUint(s string) uint {
	var n uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint(c-'0')
		}
	}
	return n
}

// parseUintSlice 将字符串切片转换为 uint 切片
func parseUintSlice(ss []string) []uint {
	result := make([]uint, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		result = append(result, parseUint(s))
	}
	return result
}
