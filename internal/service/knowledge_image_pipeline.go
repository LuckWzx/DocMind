package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"docmind/internal/model/entity"
	"docmind/pkg/docreader"
	bizerrors "docmind/pkg/errors"

	"github.com/Tencent/WeKnora/docreader/proto"
	"golang.org/x/sync/errgroup"
)

var (
	markdownImageLinkPattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)([^)]*)\)`)
	htmlImageSrcPattern      = regexp.MustCompile(`(?i)(<img\b[^>]*\bsrc=["'])([^"']+)(["'][^>]*>)`)
)

type parsedDocumentResult struct {
	MarkdownContent string
	ParserEngine    string
	ImageMappings   []StoredImage `json:"image_mappings,omitempty"`
}

func (s *knowledgeService) enrichParsedDocument(
	ctx context.Context,
	resp *proto.ReadResponse,
	knowledge *entity.Knowledge,
	requestID string,
) (*parsedDocumentResult, error) {
	markdown := strings.TrimSpace(resp.GetMarkdownContent())
	if markdown == "" {
		return nil, bizerrors.New(bizerrors.CodeInternalError, "解析结果为空")
	}

	result := &parsedDocumentResult{
		MarkdownContent: markdown,
		ParserEngine:    docreader.DetectSourceParser(resp.Metadata),
	}

	if s.imageStorage == nil || !s.imageStorage.Enabled() || len(resp.GetImageRefs()) == 0 {
		return result, nil
	}

	// 并行持久化图片
	refs := resp.GetImageRefs()
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)                                    // 最多 5 个并发
	storedResults := make([]*StoredImage, len(refs)) // 按原始索引对齐

	for i, ref := range refs {
		i, ref := i, ref
		if ref == nil {
			continue
		}
		g.Go(func() error {
			stored, err := s.persistDocImage(gctx, knowledge, requestID, ref)
			if err != nil {
				return err
			}
			storedResults[i] = stored
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "并行保存图片失败", err)
	}

	// 串行构建替换映射（避免 map 并发写入）
	replacements := make(map[string]string, len(refs)*2)
	for i, stored := range storedResults {
		if stored == nil || strings.TrimSpace(stored.URL) == "" {
			continue
		}
		ref := refs[i]
		result.ImageMappings = append(result.ImageMappings, *stored)
		for _, candidate := range buildImageReplacementCandidates(ref, stored) {
			replacements[candidate] = stored.URL
		}
	}

	if len(replacements) > 0 {
		result.MarkdownContent = rewriteImageRefsInMarkdown(result.MarkdownContent, replacements)
	}

	return result, nil
}

func (s *knowledgeService) persistDocImage(
	ctx context.Context,
	knowledge *entity.Knowledge,
	requestID string,
	ref *proto.ImageRef,
) (*StoredImage, error) {
	if ref == nil {
		return nil, nil
	}

	storageKey := strings.TrimSpace(ref.GetStorageKey())
	if len(ref.GetImageData()) == 0 {
		if isRemoteResourceURL(storageKey) {
			return &StoredImage{
				ObjectKey:   storageKey,
				URL:         storageKey,
				Filename:    ref.GetFilename(),
				OriginalRef: ref.GetOriginalRef(),
				MimeType:    ref.GetMimeType(),
			}, nil
		}
		return nil, nil
	}

	stored, err := s.imageStorage.StoreDocumentImage(ctx, &StoreDocumentImageRequest{
		KnowledgeBaseID:    knowledge.KnowledgeBaseID,
		KnowledgeID:        knowledge.ID,
		RequestID:          requestID,
		Filename:           ref.GetFilename(),
		OriginalRef:        ref.GetOriginalRef(),
		MimeType:           ref.GetMimeType(),
		SuggestedObjectKey: storageKey,
		Data:               ref.GetImageData(),
	})
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存文档图片失败", err)
	}
	return stored, nil
}

func rewriteImageRefsInMarkdown(markdown string, replacements map[string]string) string {
	if strings.TrimSpace(markdown) == "" || len(replacements) == 0 {
		return markdown
	}

	rewritten := markdownImageLinkPattern.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := markdownImageLinkPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		target, ok := lookupReplacement(parts[2], replacements)
		if !ok {
			return match
		}
		return fmt.Sprintf("![%s](%s%s)", parts[1], target, parts[3])
	})

	rewritten = htmlImageSrcPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		parts := htmlImageSrcPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		target, ok := lookupReplacement(parts[2], replacements)
		if !ok {
			return match
		}
		return parts[1] + target + parts[3]
	})

	return rewritten
}

func lookupReplacement(original string, replacements map[string]string) (string, bool) {
	candidates := make([]string, 0, 5)
	trimmed := strings.TrimSpace(strings.Trim(original, "<>\"'"))
	if trimmed != "" {
		candidates = append(candidates, trimmed)
		candidates = append(candidates, strings.ReplaceAll(trimmed, "\\", "/"))
		candidates = append(candidates, filepath.Base(trimmed))
	}

	for _, candidate := range candidates {
		if target := strings.TrimSpace(replacements[candidate]); target != "" {
			return target, true
		}
	}
	return "", false
}

func buildImageReplacementCandidates(ref *proto.ImageRef, stored *StoredImage) []string {
	values := []string{
		ref.GetOriginalRef(),
		ref.GetFilename(),
		ref.GetStorageKey(),
		stored.OriginalRef,
		stored.Filename,
		stored.ObjectKey,
	}

	candidates := make([]string, 0, len(values)*2)
	seen := make(map[string]struct{}, len(values)*2)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		normalized := strings.ReplaceAll(value, "\\", "/")
		for _, candidate := range []string{value, normalized, filepath.Base(normalized)} {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})
	return candidates
}

func isRemoteResourceURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func (s *knowledgeService) applyParsedDocumentMetadata(knowledge *entity.Knowledge, result *parsedDocumentResult) error {
	if knowledge == nil || result == nil {
		return nil
	}

	payload := map[string]any{
		"docreader": map[string]any{
			"parser_engine": result.ParserEngine,
			"image_count":   len(result.ImageMappings),
			"images":        result.ImageMappings,
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存处理元数据失败", err)
	}
	knowledge.ProcessConfig = entity.JSON(raw)
	return nil
}
