package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	"docmind/pkg/config"
	docreaderclient "docmind/pkg/docreader/client"
	bizerrors "docmind/pkg/errors"

	"github.com/Tencent/WeKnora/docreader/proto"
	"github.com/google/uuid"
)

const (
	knowledgeTypeFile         = "file"
	parseStatusPending        = "pending"
	parseStatusProcessing     = "processing"
	parseStatusCompleted      = "completed"
	parseStatusFailed         = "failed"
	knowledgeUploadTimeout    = 120 * time.Second
	knowledgeDefaultStoreRoot = "data/files"
)

type knowledgeService struct {
	knowledgeRepo     repository.KnowledgeRepository
	knowledgeBaseRepo repository.KnowledgeBaseRepository
	chunkRepo         repository.ChunkRepository
	docReaderClient   *docreaderclient.Client
	imageStorage      ImageStorageService
	cfg               *config.Config
}

// NewKnowledgeService 创建知识条目服务
func NewKnowledgeService(
	knowledgeRepo repository.KnowledgeRepository,
	knowledgeBaseRepo repository.KnowledgeBaseRepository,
	chunkRepo repository.ChunkRepository,
	docReaderClient *docreaderclient.Client,
	imageStorage ImageStorageService,
	cfg *config.Config,
) KnowledgeService {
	return &knowledgeService{
		knowledgeRepo:     knowledgeRepo,
		knowledgeBaseRepo: knowledgeBaseRepo,
		chunkRepo:         chunkRepo,
		docReaderClient:   docReaderClient,
		imageStorage:      imageStorage,
		cfg:               cfg,
	}
}

// UploadFile 上传本地文件，保存原始文件并解析为 Markdown 分块。
func (s *knowledgeService) UploadFile(userID uint, knowledgeBaseID uint, fileHeader *multipart.FileHeader) (*dto.KnowledgeFileUploadResponse, error) {
	if fileHeader == nil {
		return nil, bizerrors.New(bizerrors.CodeMissingParam, "请上传文件")
	}
	if s.docReaderClient == nil {
		return nil, bizerrors.New(bizerrors.CodeServiceUnavailable, "DocReader 服务未连接")
	}

	kb, err := s.knowledgeBaseRepo.FindByID(knowledgeBaseID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "知识库不存在")
	}

	fileName := strings.TrimSpace(fileHeader.Filename)
	if fileName == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "文件名不能为空")
	}

	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if fileType == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "无法识别文件类型")
	}

	fileBytes, err := readUploadedFile(fileHeader)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "读取上传文件失败", err)
	}
	if len(fileBytes) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "上传文件不能为空")
	}

	storedPath, err := s.storeUploadedFile(knowledgeBaseID, fileName, fileBytes)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存原始文件失败", err)
	}

	knowledge := &entity.Knowledge{
		Title:           fileName,
		FileName:        fileName,
		Type:            knowledgeTypeFile,
		ParseStatus:     parseStatusPending,
		KnowledgeBaseID: knowledgeBaseID,
		FileURL:         filepath.ToSlash(storedPath),
		FileType:        fileType,
		FileSize:        int64(len(fileBytes)),
	}
	if err := s.knowledgeRepo.Create(knowledge); err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "创建知识条目失败", err)
	}

	if err := s.updateKnowledgeStatus(knowledge, parseStatusProcessing, ""); err != nil {
		return nil, err
	}

	parseResult, err := s.parseMarkdown(fileBytes, fileName, fileType, kb, knowledge)
	if err != nil {
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return nil, err
	}
	if err := s.applyParsedDocumentMetadata(knowledge, parseResult); err != nil {
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return nil, err
	}

	chunks, err := s.buildMarkdownChunks(parseResult.MarkdownContent, parseResult.ParserEngine, kb, knowledge)
	if err != nil {
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return nil, err
	}
	if err := s.chunkRepo.CreateBatch(chunks); err != nil {
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, "保存分块失败")
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "保存分块失败", err)
	}

	if err := s.updateKnowledgeStatus(knowledge, parseStatusCompleted, ""); err != nil {
		return nil, err
	}

	return &dto.KnowledgeFileUploadResponse{
		KnowledgeID:     knowledge.ID,
		KnowledgeBaseID: knowledge.KnowledgeBaseID,
		Title:           knowledge.Title,
		FileType:        knowledge.FileType,
		FilePath:        knowledge.FileURL,
		ParseStatus:     knowledge.ParseStatus,
		ChunkCount:      len(chunks),
		MarkdownChars:   len([]rune(parseResult.MarkdownContent)),
	}, nil
}

func (s *knowledgeService) parseMarkdown(
	fileBytes []byte,
	fileName, fileType string,
	kb *entity.KnowledgeBase,
	knowledge *entity.Knowledge,
) (*parsedDocumentResult, error) {
	request := &proto.ReadRequest{
		FileContent: fileBytes,
		FileName:    fileName,
		FileType:    fileType,
		RequestId:   uuid.NewString(),
		Title:       fileName,
	}
	if parserEngine := selectParserEngine(kb.ChunkingConfig.ParserEngineRules, fileType); parserEngine != "" {
		request.Config = &proto.ReadConfig{
			ParserEngine: parserEngine,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), knowledgeUploadTimeout)
	defer cancel()

	resp, err := s.docReaderClient.Read(ctx, request)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeServiceUnavailable, "DocReader 解析失败", err)
	}
	if resp == nil {
		return nil, bizerrors.New(bizerrors.CodeServiceUnavailable, "DocReader 返回为空")
	}
	if strings.TrimSpace(resp.Error) != "" {
		return nil, bizerrors.New(bizerrors.CodeServiceUnavailable, strings.TrimSpace(resp.Error))
	}

	result, err := s.enrichParsedDocument(ctx, resp, knowledge, request.RequestId)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *knowledgeService) buildMarkdownChunks(markdownContent string, parserEngine string, kb *entity.KnowledgeBase, knowledge *entity.Knowledge) ([]*entity.Chunk, error) {
	cfg := normalizeChunkingConfig(kb.ChunkingConfig)
	profile := buildDocProfile(markdownContent, cfg.Languages)
	selectedTier, _, _ := selectChunkingTier(cfg, profile)
	units := splitPreviewUnits(markdownContent, cfg, selectedTier)
	if len(units) == 0 {
		units = []chunkUnit{{Content: markdownContent}}
	}

	chunks := make([]*entity.Chunk, 0, len(units))
	for index, unit := range units {
		content := strings.TrimSpace(unit.Content)
		if content == "" {
			continue
		}

		metadata := entity.ChunkMetadata{
			DocTitle:      knowledge.Title,
			ContextHeader: strings.TrimSpace(unit.ContextHeader),
			SourceFormat:  knowledge.FileType,
			SourceParser:  parserEngine,
		}
		metadataRaw, err := json.Marshal(metadata)
		if err != nil {
			return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "构建分块元数据失败", err)
		}

		chunks = append(chunks, &entity.Chunk{
			Content:         content,
			ChunkIndex:      index,
			KnowledgeID:     knowledge.ID,
			KnowledgeBaseID: knowledge.KnowledgeBaseID,
			ChunkType:       entity.ChunkTypeMarkdown,
			ChunkStatus:     1,
			Metadata:        entity.JSON(metadataRaw),
			ContentHash:     sha256Hex(content),
			IsEnabled:       true,
		})
	}
	if len(chunks) == 0 {
		return nil, bizerrors.New(bizerrors.CodeInternalError, "未生成有效分块")
	}
	return chunks, nil
}

func (s *knowledgeService) storeUploadedFile(knowledgeBaseID uint, fileName string, fileBytes []byte) (string, error) {
	root := knowledgeDefaultStoreRoot
	if s.cfg != nil && strings.TrimSpace(s.cfg.Storage.LocalRoot) != "" {
		root = strings.TrimSpace(s.cfg.Storage.LocalRoot)
	}

	extension := filepath.Ext(fileName)
	storedName := uuid.NewString() + extension
	relativePath := filepath.Join(root, fmt.Sprintf("%d", knowledgeBaseID), storedName)
	if err := os.MkdirAll(filepath.Dir(relativePath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(relativePath, fileBytes, 0o644); err != nil {
		return "", err
	}
	return relativePath, nil
}

func (s *knowledgeService) updateKnowledgeStatus(knowledge *entity.Knowledge, status, errorMessage string) error {
	knowledge.ParseStatus = status
	knowledge.ErrorMessage = errorMessage
	if err := s.knowledgeRepo.Update(knowledge); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新知识状态失败", err)
	}
	return nil
}

func readUploadedFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func selectParserEngine(rules []entity.ParserEngineRule, fileType string) string {
	normalizedType := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
	if normalizedType == "" {
		return ""
	}
	for _, rule := range rules {
		for _, candidate := range rule.FileTypes {
			current := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(candidate)), ".")
			if current == normalizedType {
				return strings.TrimSpace(rule.Engine)
			}
		}
	}
	return ""
}

func detectSourceParser(metadata map[string]string) string {
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

func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
