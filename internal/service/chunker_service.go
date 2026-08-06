package service

import (
	"math"
	"regexp"
	"sort"
	"strings"

	req "docmind/internal/model/dto/request"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
)

const (
	chunkerDefaultSize         = 512
	chunkerDefaultOverlap      = 80
	chunkerDefaultParentSize   = 4096
	chunkerDefaultChildSize    = 384
	chunkerDefaultTokenLimit   = 0
	chunkerStrategyAuto        = "auto"
	chunkerStrategyHeading     = "heading"
	chunkerStrategyHeuristic   = "heuristic"
	chunkerStrategyLegacy      = "legacy"
	chunkerPreviewMaxChunkShow = 200
)

var (
	markdownHeadingPattern = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+?)\s*$`)
	numberedHeadingPattern = regexp.MustCompile(`(?m)^\s*\d+(?:\.\d+){0,3}[\.、)]?\s+\S+`)
	visualSeparatorPattern = regexp.MustCompile(`(?m)^[-_=]{3,}\s*$`)
	chinesePattern         = regexp.MustCompile(`[\p{Han}]`)
	englishPattern         = regexp.MustCompile(`[A-Za-z]`)
	germanPattern          = regexp.MustCompile(`[ÄÖÜäöüß]`)

	// 受保护内容模式：分块时保持整体，避免路径/引用被切断
	protectedInlinePattern      = regexp.MustCompile(`!?\[[^\]]*\]\([^)]*\)`)                 // Markdown 图片/链接
	protectedCodeFencePattern   = regexp.MustCompile("(?s)```.*?```")                         // 成对代码块
	protectedTableRowPattern    = regexp.MustCompile(`(?m)^[ \t]*\|[^\n]*\|[ \t]*$`)          // 表格行
	protectedURLPattern         = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s)\]}]+`) // URL
	protectedWindowsPathPattern = regexp.MustCompile(`[A-Za-z]:\\[^\s)\]}]+`)                 // Windows 路径

	// 结构化文本标题模式（无 Markdown # 语法时的标题识别，如 PDF 解析结果）
	chineseChapterPattern = regexp.MustCompile(`^第[一二三四五六七八九十百千0-9]+[章节部分篇]`)
	englishChapterPattern = regexp.MustCompile(`(?i)^chapter\s+\d+`)
	germanChapterPattern  = regexp.MustCompile(`(?i)^kapitel\s+\d+`)
	numberedTitlePattern  = regexp.MustCompile(`^\d+(?:\.\d+){0,3}[\.、)]?\s+\S`)
	numberedPrefixPattern = regexp.MustCompile(`^\d+(?:\.\d+){0,3}`)
	allCapsTitlePattern   = regexp.MustCompile(`^[A-Z][A-Z0-9 &'\-:()/.,]{1,38}$`)
)

type chunkerService struct{}

type chunkUnit struct {
	Content       string
	ContextHeader string
}

type headingSection struct {
	HeadingPath []string
	Content     string
}

// textPiece 分块原子段：protected=true 表示受保护内容（不可切分）
type textPiece struct {
	content   string
	protected bool
}

// headingLine 标题行（Markdown # 标题或结构化文本标题）
type headingLine struct {
	start int    // 标题在原文中的起始字节位置
	level int    // 标题层级（1~6）
	title string // 标题文本
}

// NewChunkerService 创建 Markdown 分块服务
func NewChunkerService() ChunkerService {
	return &chunkerService{}
}

// Preview 预览分块效果，优先返回最终会参与向量化的叶子块。
func (s *chunkerService) Preview(request *req.PreviewChunkingRequest) (*dto.PreviewChunkingResponse, error) {
	text := strings.ReplaceAll(strings.TrimSpace(request.Text), "\r\n", "\n")
	cfg := normalizeChunkingConfig(request.ChunkingConfig)
	profile := buildDocProfile(text, cfg.Languages)
	selectedTier, tierChain, rejected := selectChunkingTier(cfg, profile)
	units := splitPreviewUnits(text, cfg, selectedTier)
	chunks := buildPreviewChunks(units)
	stats := buildPreviewStats(chunks)

	return &dto.PreviewChunkingResponse{
		SelectedTier: selectedTier,
		TierChain:    tierChain,
		Rejected:     rejected,
		Profile:      profile,
		Chunks:       chunks,
		Stats:        stats,
	}, nil
}

func normalizeChunkingConfig(cfg entity.ChunkingConfig) entity.ChunkingConfig {
	if cfg.ChunkSize < 100 {
		cfg.ChunkSize = chunkerDefaultSize
	}
	if cfg.ChunkSize > 4000 {
		cfg.ChunkSize = 4000
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = 0
	}
	if cfg.ChunkOverlap == 0 {
		cfg.ChunkOverlap = chunkerDefaultOverlap
	}
	if maxOverlap := cfg.ChunkSize / 2; cfg.ChunkOverlap > maxOverlap {
		cfg.ChunkOverlap = maxOverlap
	}
	if len(cfg.Separators) == 0 {
		cfg.Separators = []string{"\n\n", "\n", "。", "！", "？", ";", "；", " "}
	}
	if strings.TrimSpace(cfg.Strategy) == "" {
		cfg.Strategy = chunkerStrategyAuto
	}
	if cfg.TokenLimit < 0 {
		cfg.TokenLimit = chunkerDefaultTokenLimit
	}
	if cfg.ParentChunkSize < 512 {
		cfg.ParentChunkSize = chunkerDefaultParentSize
	}
	if cfg.ParentChunkSize > 8192 {
		cfg.ParentChunkSize = 8192
	}
	if cfg.ChildChunkSize < 64 {
		cfg.ChildChunkSize = chunkerDefaultChildSize
	}
	if cfg.ChildChunkSize > 2048 {
		cfg.ChildChunkSize = 2048
	}
	return cfg
}

func buildDocProfile(text string, languageHints []string) dto.DocProfile {
	lines := strings.Split(text, "\n")
	lineLens := make([]float64, 0, len(lines))
	headingCounts := make(map[string]int)
	codeFenceCount := strings.Count(text, "```")

	var totalChars int
	var allCapsShortLineCount int
	for _, line := range lines {
		length := runeLen(line)
		totalChars += length
		lineLens = append(lineLens, float64(length))
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) <= 40 && trimmed == strings.ToUpper(trimmed) && englishPattern.MatchString(trimmed) {
			allCapsShortLineCount++
		}
	}

	for _, match := range markdownHeadingPattern.FindAllStringSubmatch(text, -1) {
		level := match[1]
		headingCounts[level]++
	}

	detectedLangSet := map[string]struct{}{}
	for _, hint := range languageHints {
		if trimmed := strings.TrimSpace(hint); trimmed != "" {
			detectedLangSet[trimmed] = struct{}{}
		}
	}
	if chinesePattern.MatchString(text) {
		detectedLangSet["zh"] = struct{}{}
	}
	if englishPattern.MatchString(text) {
		detectedLangSet["en"] = struct{}{}
	}
	if germanPattern.MatchString(text) {
		detectedLangSet["de"] = struct{}{}
	}

	detectedLangs := make([]string, 0, len(detectedLangSet))
	for lang := range detectedLangSet {
		detectedLangs = append(detectedLangs, lang)
	}
	sort.Strings(detectedLangs)

	return dto.DocProfile{
		TotalChars:            totalChars,
		TotalLines:            len(lines),
		AvgLineLen:            mean(lineLens),
		StdLineLen:            stddev(lineLens),
		MDHeadingCounts:       headingCounts,
		MDHeadingTotal:        len(markdownHeadingPattern.FindAllString(text, -1)),
		NumberedSectionCount:  len(numberedHeadingPattern.FindAllString(text, -1)),
		AllCapsShortLineCount: allCapsShortLineCount,
		BlankParagraphBreaks:  strings.Count(text, "\n\n"),
		FormFeedCount:         strings.Count(text, "\f"),
		VisualSepCount:        len(visualSeparatorPattern.FindAllString(text, -1)),
		GermanChapterCount:    len(regexp.MustCompile(`(?m)^kapitel\s+\d+`).FindAllString(strings.ToLower(text), -1)),
		EnglishChapterCount:   len(regexp.MustCompile(`(?m)^chapter\s+\d+`).FindAllString(strings.ToLower(text), -1)),
		ChineseChapterCount:   len(regexp.MustCompile(`(?m)^第[一二三四五六七八九十百千0-9]+[章节部分篇]`).FindAllString(text, -1)),
		RepeatedFooterCount:   estimateRepeatedFooterCount(lines),
		HasTables:             strings.Contains(text, "|") && strings.Contains(text, "\n|"),
		HasCode:               codeFenceCount > 0,
		CodeRatio:             safeDivide(float64(codeFenceCount*3), float64(max(totalChars, 1))),
		DetectedLangs:         detectedLangs,
	}
}

func selectChunkingTier(cfg entity.ChunkingConfig, profile dto.DocProfile) (dto.StrategyTier, []dto.StrategyTier, []dto.TierRejection) {
	switch strings.ToLower(strings.TrimSpace(cfg.Strategy)) {
	case chunkerStrategyHeading:
		return dto.StrategyTierHeading, []dto.StrategyTier{dto.StrategyTierHeading}, nil
	case chunkerStrategyHeuristic:
		return dto.StrategyTierHeuristic, []dto.StrategyTier{dto.StrategyTierHeuristic}, nil
	case chunkerStrategyLegacy:
		return dto.StrategyTierLegacy, []dto.StrategyTier{dto.StrategyTierLegacy}, nil
	}

	tierChain := []dto.StrategyTier{dto.StrategyTierHeading, dto.StrategyTierHeuristic, dto.StrategyTierLegacy}
	rejected := make([]dto.TierRejection, 0, 2)
	// auto 策略：检测到足够的结构化标题信号（Markdown # 标题、中/英/德章节、
	// 编号标题、全大写短行）时按标题切分
	chapterSignals := profile.MDHeadingTotal +
		profile.ChineseChapterCount + profile.EnglishChapterCount + profile.GermanChapterCount
	if chapterSignals >= 2 || profile.NumberedSectionCount >= 3 ||
		(profile.AllCapsShortLineCount >= 3 && profile.AllCapsShortLineCount > profile.RepeatedFooterCount) {
		rejected = append(rejected,
			dto.TierRejection{Tier: dto.StrategyTierHeuristic, Reason: "auto 检测到稳定标题结构，优先按标题切分"},
			dto.TierRejection{Tier: dto.StrategyTierLegacy, Reason: "已命中更高优先级的结构化策略"},
		)
		return dto.StrategyTierHeading, tierChain, rejected
	}
	if profile.BlankParagraphBreaks > 0 || profile.NumberedSectionCount > 1 || profile.VisualSepCount > 0 {
		rejected = append(rejected,
			dto.TierRejection{Tier: dto.StrategyTierHeading, Reason: "标题信号不足，回退到结构感知切分"},
			dto.TierRejection{Tier: dto.StrategyTierLegacy, Reason: "auto 仍可利用段落或编号结构，不需要纯长度切分"},
		)
		return dto.StrategyTierHeuristic, tierChain, rejected
	}
	rejected = append(rejected,
		dto.TierRejection{Tier: dto.StrategyTierHeading, Reason: "未检测到足够的标题结构"},
		dto.TierRejection{Tier: dto.StrategyTierHeuristic, Reason: "段落或编号结构信号较弱，回退到长度优先策略"},
	)
	return dto.StrategyTierLegacy, tierChain, rejected
}

// splitUnits 按配置切分文本为分块单元，不限制数量（入库链路使用）
func splitUnits(text string, cfg entity.ChunkingConfig, tier dto.StrategyTier) []chunkUnit {
	if text == "" {
		return nil
	}
	if cfg.EnableParentChild {
		parents := splitTierUnits(text, cfg, tier, cfg.ParentChunkSize, 0)
		children := make([]chunkUnit, 0, len(parents)*2)
		for _, parent := range parents {
			for _, child := range splitRecursive(parent.Content, cfg.ChildChunkSize, cfg.ChunkOverlap, cfg.Separators) {
				children = append(children, chunkUnit{
					Content:       child,
					ContextHeader: parent.ContextHeader,
				})
			}
		}
		return children
	}
	return splitTierUnits(text, cfg, tier, cfg.ChunkSize, cfg.ChunkOverlap)
}

// splitPreviewUnits 预览分块效果，限制返回数量（预览接口使用）
func splitPreviewUnits(text string, cfg entity.ChunkingConfig, tier dto.StrategyTier) []chunkUnit {
	return limitPreviewUnits(splitUnits(text, cfg, tier))
}

func splitTierUnits(text string, cfg entity.ChunkingConfig, tier dto.StrategyTier, size int, overlap int) []chunkUnit {
	switch tier {
	case dto.StrategyTierHeading:
		return splitHeadingUnits(text, size, overlap, cfg.Separators)
	case dto.StrategyTierHeuristic:
		return splitHeuristicUnits(text, size, overlap, cfg.Separators)
	default:
		return wrapRecursiveUnits(splitRecursive(text, size, overlap, cfg.Separators), "")
	}
}

func splitHeadingUnits(text string, size int, overlap int, separators []string) []chunkUnit {
	sections := parseMarkdownSections(text)
	if len(sections) == 0 {
		return wrapRecursiveUnits(splitRecursive(text, size, overlap, separators), "")
	}

	units := make([]chunkUnit, 0, len(sections))
	for _, section := range sections {
		header := strings.Join(section.HeadingPath, " / ")
		for _, piece := range splitRecursive(section.Content, size, overlap, separators) {
			units = append(units, chunkUnit{
				Content:       piece,
				ContextHeader: header,
			})
		}
	}
	return units
}

func splitHeuristicUnits(text string, size int, overlap int, separators []string) []chunkUnit {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) <= 1 {
		return wrapRecursiveUnits(splitRecursive(text, size, overlap, separators), "")
	}
	return wrapRecursiveUnits(packSegments(paragraphs, size, overlap), "")
}

func parseMarkdownSections(text string) []headingSection {
	headings := findHeadingLines(text)
	if len(headings) == 0 {
		return nil
	}

	sections := make([]headingSection, 0, len(headings))
	stack := make([]string, 0, 6)
	for idx, h := range headings {
		for len(stack) >= h.level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, h.title)

		start := h.start
		end := len(text)
		if idx+1 < len(headings) {
			end = headings[idx+1].start
		}
		content := strings.TrimSpace(text[start:end])
		if content == "" {
			continue
		}
		path := append([]string(nil), stack...)
		sections = append(sections, headingSection{
			HeadingPath: path,
			Content:     content,
		})
	}
	return sections
}

// findHeadingLines 识别文档标题行：优先 Markdown # 标题，无 # 标题时
// 回退识别结构化文本标题（中文章节/英文章节/编号标题/全大写短行）
func findHeadingLines(text string) []headingLine {
	if lines := findMarkdownHeadingLines(text); len(lines) > 0 {
		return lines
	}
	return findTextHeadingLines(text)
}

func findMarkdownHeadingLines(text string) []headingLine {
	matches := markdownHeadingPattern.FindAllStringSubmatchIndex(text, -1)
	lines := make([]headingLine, 0, len(matches))
	for _, m := range matches {
		lines = append(lines, headingLine{
			start: m[0],
			level: len(text[m[2]:m[3]]),
			title: strings.TrimSpace(text[m[4]:m[5]]),
		})
	}
	return lines
}

// findTextHeadingLines 识别无 Markdown 语法的结构化标题（PDF 等解析结果常见）。
// 全大写短行需排除重复行，避免把页眉页脚当作标题。
func findTextHeadingLines(text string) []headingLine {
	rawLines := strings.Split(text, "\n")
	upperCounts := make(map[string]int)
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && runeLen(trimmed) <= 40 && allCapsTitlePattern.MatchString(trimmed) {
			upperCounts[trimmed]++
		}
	}

	var headings []headingLine
	offset := 0
	for i, line := range rawLines {
		start := offset
		if i < len(rawLines)-1 {
			offset += len(line) + 1
		} else {
			offset = len(text)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		title := ""
		level := 1
		switch {
		case chineseChapterPattern.MatchString(trimmed):
			title = trimmed
		case englishChapterPattern.MatchString(trimmed), germanChapterPattern.MatchString(trimmed):
			title = trimmed
		case numberedTitlePattern.MatchString(trimmed):
			title = trimmed
			level = 1 + strings.Count(numberedPrefixPattern.FindString(trimmed), ".")
		case upperCounts[trimmed] == 1 && allCapsTitlePattern.MatchString(trimmed):
			title = trimmed
		default:
			continue
		}
		headings = append(headings, headingLine{start: start, level: level, title: title})
	}
	return headings
}

func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	parts := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitRecursive(text string, size int, overlap int, separators []string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	if runeLen(trimmed) <= size {
		return []string{trimmed}
	}

	// 受保护内容（图片/链接/代码块/表格）作为不可切分单元，避免被拦腰切断
	if pieces := splitProtectedPieces(trimmed); len(pieces) > 1 {
		segments := make([]string, 0, len(pieces))
		for _, piece := range pieces {
			if piece.protected || runeLen(piece.content) <= size {
				segments = append(segments, piece.content)
			} else {
				segments = append(segments, splitRecursive(piece.content, size, overlap, separators)...)
			}
		}
		return packSegments(segments, size, overlap)
	}

	for _, separator := range separators {
		if separator == "" || !strings.Contains(trimmed, separator) {
			continue
		}
		segments := splitKeepingSeparator(trimmed, separator)
		if len(segments) <= 1 {
			continue
		}
		packed := packSegments(segments, size, overlap)
		if len(packed) > 1 {
			return packed
		}
	}
	return splitByRunes(trimmed, size, overlap)
}

func splitKeepingSeparator(text string, separator string) []string {
	raw := strings.Split(text, separator)
	if len(raw) <= 1 {
		return nil
	}
	parts := make([]string, 0, len(raw))
	for index, item := range raw {
		if index < len(raw)-1 {
			item += separator
		}
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// packSegments 会尽量保留段落或句子边界，只有在确实超限时才退回定长切片。
func packSegments(segments []string, size int, overlap int) []string {
	chunks := make([]string, 0, len(segments))
	var current strings.Builder
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		candidate := segment
		if current.Len() > 0 {
			candidate = current.String() + "\n\n" + segment
		}
		if runeLen(candidate) <= size {
			current.Reset()
			current.WriteString(candidate)
			continue
		}

		if current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			tail := tailRunes(current.String(), overlap)
			current.Reset()
			if tail != "" {
				current.WriteString(tail)
				current.WriteString("\n\n")
			}
		}

		if runeLen(segment) <= size {
			current.WriteString(segment)
			continue
		}
		chunks = append(chunks, splitByRunes(segment, size, overlap)...)
	}
	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	return compactChunks(chunks)
}

func splitByRunes(text string, size int, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		return []string{strings.TrimSpace(text)}
	}
	step := size - overlap
	if step <= 0 {
		step = size
	}

	// 受保护片段区间（rune 下标），切分点尽量避开，避免切断路径/引用
	protected := protectedRunesRanges(text, size)

	chunks := make([]string, 0, int(math.Ceil(float64(len(runes))/float64(step))))
	start := 0
	for start < len(runes) {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		// 切分点落在受保护区间内时，前移到区间起点，保证片段完整
		if segStart, ok := protectedStartBefore(protected, end); ok && segStart > start && segStart < end {
			end = segStart
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
		next := end - overlap
		if next <= start {
			next = end // 避开受保护片段时放弃重叠，保证向前推进
		}
		start = next
	}
	return compactChunks(chunks)
}

func wrapRecursiveUnits(chunks []string, header string) []chunkUnit {
	units := make([]chunkUnit, 0, len(chunks))
	for _, chunk := range chunks {
		units = append(units, chunkUnit{
			Content:       chunk,
			ContextHeader: header,
		})
	}
	return units
}

func buildPreviewChunks(units []chunkUnit) []dto.PreviewChunk {
	chunks := make([]dto.PreviewChunk, 0, len(units))
	offset := 0
	for index, unit := range units {
		sizeChars := runeLen(unit.Content)
		chunks = append(chunks, dto.PreviewChunk{
			Seq:              index + 1,
			Start:            offset,
			End:              offset + sizeChars,
			SizeChars:        sizeChars,
			SizeTokensApprox: int(math.Ceil(float64(sizeChars) / 4)),
			ContextHeader:    unit.ContextHeader,
			Content:          unit.Content,
		})
		offset += sizeChars
	}
	return chunks
}

func buildPreviewStats(chunks []dto.PreviewChunk) dto.PreviewChunkingStats {
	if len(chunks) == 0 {
		return dto.PreviewChunkingStats{}
	}
	sizes := make([]float64, 0, len(chunks))
	minChars := chunks[0].SizeChars
	maxChars := chunks[0].SizeChars
	for _, chunk := range chunks {
		sizes = append(sizes, float64(chunk.SizeChars))
		if chunk.SizeChars < minChars {
			minChars = chunk.SizeChars
		}
		if chunk.SizeChars > maxChars {
			maxChars = chunk.SizeChars
		}
	}
	return dto.PreviewChunkingStats{
		Count:       len(chunks),
		AvgChars:    mean(sizes),
		MinChars:    minChars,
		MaxChars:    maxChars,
		StddevChars: stddev(sizes),
	}
}

func limitPreviewUnits(units []chunkUnit) []chunkUnit {
	if len(units) <= chunkerPreviewMaxChunkShow {
		return units
	}
	return units[:chunkerPreviewMaxChunkShow]
}

func compactChunks(chunks []string) []string {
	result := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if trimmed := strings.TrimSpace(chunk); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func tailRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return string(runes)
	}
	cutoff := len(runes) - limit
	// 截断点落在受保护区间内时前移到区间起点，避免 overlap 处切断路径/引用
	if segStart, ok := protectedStartBefore(protectedRunesRanges(trimmed, limit*2), cutoff); ok && segStart < cutoff {
		cutoff = segStart
	}
	return string(runes[cutoff:])
}

// protectedRanges 提取受保护内容的字节区间，合并重叠或仅隔换行的相邻片段
// （连续表格行合并为一个整体）
func protectedRanges(text string) [][2]int {
	var ranges [][2]int
	collect := func(pattern *regexp.Regexp) {
		for _, m := range pattern.FindAllStringSubmatchIndex(text, -1) {
			ranges = append(ranges, [2]int{m[0], m[1]})
		}
	}
	collect(protectedInlinePattern)
	collect(protectedCodeFencePattern)
	collect(protectedTableRowPattern)
	collect(protectedURLPattern)
	collect(protectedWindowsPathPattern)
	if len(ranges) == 0 {
		return nil
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })

	merged := make([][2]int, 0, len(ranges))
	cur := ranges[0]
	for _, r := range ranges[1:] {
		if r[0] <= cur[1]+2 { // 重叠或仅隔一个换行（含 \r\n）
			if r[1] > cur[1] {
				cur[1] = r[1]
			}
		} else {
			merged = append(merged, cur)
			cur = r
		}
	}
	return append(merged, cur)
}

// splitProtectedPieces 将文本按受保护内容拆分为原子段序列（无保护内容时返回 nil）
func splitProtectedPieces(text string) []textPiece {
	ranges := protectedRanges(text)
	if len(ranges) == 0 {
		return nil
	}
	pieces := make([]textPiece, 0, len(ranges)*2+1)
	last := 0
	for _, r := range ranges {
		if r[0] > last {
			pieces = append(pieces, textPiece{content: text[last:r[0]]})
		}
		pieces = append(pieces, textPiece{content: text[r[0]:r[1]], protected: true})
		last = r[1]
	}
	if last < len(text) {
		pieces = append(pieces, textPiece{content: text[last:]})
	}
	return pieces
}

// protectedRunesRanges 返回受保护片段的 rune 下标区间，超长片段（>= maxLen）不保护
func protectedRunesRanges(text string, maxLen int) [][2]int {
	byteRanges := protectedRanges(text)
	if len(byteRanges) == 0 {
		return nil
	}
	ranges := make([][2]int, 0, len(byteRanges))
	for _, r := range byteRanges {
		start, end := runeIndexAt(text, r[0]), runeIndexAt(text, r[1])
		if end-start >= maxLen {
			continue // 超长片段无法整体保留，允许硬切
		}
		ranges = append(ranges, [2]int{start, end})
	}
	return ranges
}

// protectedStartBefore 若 cutoff（rune 下标）落在受保护区间内，返回区间起点
func protectedStartBefore(ranges [][2]int, cutoff int) (int, bool) {
	for _, r := range ranges {
		if cutoff > r[0] && cutoff < r[1] {
			return r[0], true
		}
	}
	return 0, false
}

// runeIndexAt 将字节下标换算为 rune 下标
func runeIndexAt(text string, bytePos int) int {
	if bytePos <= 0 {
		return 0
	}
	if bytePos >= len(text) {
		return len([]rune(text))
	}
	return len([]rune(text[:bytePos]))
}

func estimateRepeatedFooterCount(lines []string) int {
	if len(lines) < 6 {
		return 0
	}
	counts := make(map[string]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || runeLen(trimmed) > 40 {
			continue
		}
		counts[trimmed]++
	}
	total := 0
	for _, count := range counts {
		if count > 2 {
			total += count
		}
	}
	return total
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	avg := mean(values)
	var sum float64
	for _, value := range values {
		diff := value - avg
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func runeLen(text string) int {
	return len([]rune(text))
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
