package response

// StrategyTier 分块策略层级
type StrategyTier string

const (
	StrategyTierHeading   StrategyTier = "heading"
	StrategyTierHeuristic StrategyTier = "heuristic"
	StrategyTierRecursive StrategyTier = "recursive"
	StrategyTierLegacy    StrategyTier = "legacy"
)

// TierRejection 被拒绝的策略及原因
type TierRejection struct {
	Tier   StrategyTier `json:"tier"`
	Reason string       `json:"reason"`
}

// DocProfile 文档画像，用于帮助前端解释为何选中某种策略
type DocProfile struct {
	TotalChars            int            `json:"total_chars"`
	TotalLines            int            `json:"total_lines"`
	AvgLineLen            float64        `json:"avg_line_len"`
	StdLineLen            float64        `json:"std_line_len"`
	MDHeadingCounts       map[string]int `json:"md_heading_counts"`
	MDHeadingTotal        int            `json:"md_heading_total"`
	NumberedSectionCount  int            `json:"numbered_section_count"`
	AllCapsShortLineCount int            `json:"all_caps_short_line_count"`
	BlankParagraphBreaks  int            `json:"blank_paragraph_breaks"`
	FormFeedCount         int            `json:"form_feed_count"`
	VisualSepCount        int            `json:"visual_sep_count"`
	GermanChapterCount    int            `json:"german_chapter_count"`
	EnglishChapterCount   int            `json:"english_chapter_count"`
	ChineseChapterCount   int            `json:"chinese_chapter_count"`
	RepeatedFooterCount   int            `json:"repeated_footer_count"`
	HasTables             bool           `json:"has_tables"`
	HasCode               bool           `json:"has_code"`
	CodeRatio             float64        `json:"code_ratio"`
	DetectedLangs         []string       `json:"detected_langs"`
}

// PreviewChunk 分块预览结果
type PreviewChunk struct {
	Seq              int    `json:"seq"`
	Start            int    `json:"start"`
	End              int    `json:"end"`
	SizeChars        int    `json:"size_chars"`
	SizeTokensApprox int    `json:"size_tokens_approx"`
	ContextHeader    string `json:"context_header,omitempty"`
	Content          string `json:"content"`
}

// PreviewChunkingStats 分块统计信息
type PreviewChunkingStats struct {
	Count       int     `json:"count"`
	AvgChars    float64 `json:"avg_chars"`
	MinChars    int     `json:"min_chars"`
	MaxChars    int     `json:"max_chars"`
	StddevChars float64 `json:"stddev_chars"`
	TruncatedTo int     `json:"truncated_to,omitempty"`
}

// PreviewChunkingResponse 分块预览响应
type PreviewChunkingResponse struct {
	SelectedTier StrategyTier         `json:"selected_tier"`
	TierChain    []StrategyTier       `json:"tier_chain"`
	Rejected     []TierRejection      `json:"rejected"`
	Profile      DocProfile           `json:"profile"`
	Chunks       []PreviewChunk       `json:"chunks"`
	Stats        PreviewChunkingStats `json:"stats"`
}
