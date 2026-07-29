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
	if profile.MDHeadingTotal >= 2 {
		rejected = append(rejected,
			dto.TierRejection{Tier: dto.StrategyTierHeuristic, Reason: "auto 检测到稳定 Markdown 标题结构，优先按标题切分"},
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

func splitPreviewUnits(text string, cfg entity.ChunkingConfig, tier dto.StrategyTier) []chunkUnit {
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
		return limitPreviewUnits(children)
	}
	return limitPreviewUnits(splitTierUnits(text, cfg, tier, cfg.ChunkSize, cfg.ChunkOverlap))
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
	matches := markdownHeadingPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	sections := make([]headingSection, 0, len(matches))
	stack := make([]string, 0, 6)
	for idx, match := range matches {
		level := len(text[match[2]:match[3]])
		title := strings.TrimSpace(text[match[4]:match[5]])
		for len(stack) >= level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, title)

		start := match[0]
		end := len(text)
		if idx+1 < len(matches) {
			end = matches[idx+1][0]
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

	chunks := make([]string, 0, int(math.Ceil(float64(len(runes))/float64(step))))
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
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
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[len(runes)-limit:])
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
