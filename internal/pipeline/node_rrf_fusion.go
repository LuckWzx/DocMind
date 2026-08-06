package pipeline

import (
	"context"
	"fmt"
	"sort"
)

// rrfK RRF 平滑常数（标准经验值，所有 RRF 实现统一使用）
const rrfK = 60

// newRRFFusionNode 创建 RRF 融合节点
// 将向量检索（SearchResults）与 BM25 关键词检索（KeywordResults）两路结果
// 按倒数排名融合为一路：RRF(d) = Σ 1/(k + rank_i(d))。
// 只比排名不比分数（两路分数量纲不同），天然完成去重，融合结果写回 SearchResults，
// 下游 rerank/build_prompt 节点零改动。
func newRRFFusionNode() func(ctx context.Context, input *Context) (*Context, error) {
	return func(ctx context.Context, input *Context) (*Context, error) {
		vectorResults := input.SearchResults
		keywordResults := input.KeywordResults

		// 双路都为空 → 空结果
		if len(vectorResults) == 0 && len(keywordResults) == 0 {
			input.SearchResults = []SearchResult{}
			return input, nil
		}
		// 单路为空 → 直接使用另一路
		if len(vectorResults) == 0 {
			input.SearchResults = keywordResults
			return input, nil
		}
		if len(keywordResults) == 0 {
			input.SearchResults = vectorResults
			return input, nil
		}

		// 1. 按排名累加 RRF 分数（同一 chunk 双路命中自动合并贡献）
		fused := make(map[uint]float64, len(vectorResults)+len(keywordResults))
		for rank, r := range vectorResults {
			fused[r.ChunkID] += 1.0 / (rrfK + float64(rank+1))
		}
		for rank, r := range keywordResults {
			fused[r.ChunkID] += 1.0 / (rrfK + float64(rank+1))
		}

		// 2. 还原条目信息（双路命中时优先取向量路条目，内容一致）
		entries := make(map[uint]SearchResult, len(vectorResults)+len(keywordResults))
		for _, r := range vectorResults {
			entries[r.ChunkID] = r
		}
		for _, r := range keywordResults {
			if _, ok := entries[r.ChunkID]; !ok {
				entries[r.ChunkID] = r
			}
		}

		// 3. 按 RRF 分数降序排序
		chunkIDs := make([]uint, 0, len(fused))
		for id := range fused {
			chunkIDs = append(chunkIDs, id)
		}
		sort.Slice(chunkIDs, func(i, j int) bool {
			return fused[chunkIDs[i]] > fused[chunkIDs[j]]
		})

		// 4. 截断到 TopK
		topK := input.AgentConfig.RerankTopK
		if topK <= 0 {
			topK = 5
		}
		if len(chunkIDs) > topK {
			chunkIDs = chunkIDs[:topK]
		}

		// 5. 写回 SearchResults（Score 替换为 RRF 分数，下游 rerank 会重新打分）
		results := make([]SearchResult, 0, len(chunkIDs))
		for _, id := range chunkIDs {
			item := entries[id]
			item.Score = fused[id]
			results = append(results, item)
		}

		input.SearchResults = results
		fmt.Printf("[Pipeline] RRFFusion: vector=%d + keyword=%d → fused=%d (topK=%d)\n",
			len(vectorResults), len(keywordResults), len(results), topK)

		return input, nil
	}
}
