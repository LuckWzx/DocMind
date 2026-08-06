package service

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docmind/internal/llm"
	dto "docmind/internal/model/dto/response"
	"docmind/internal/model/entity"
	"docmind/internal/repository"
	"docmind/pkg/config"
	"docmind/pkg/docreader"
	docreaderclient "docmind/pkg/docreader/client"
	bizerrors "docmind/pkg/errors"
	"docmind/pkg/fileutil"
	"docmind/pkg/logger"

	"github.com/Tencent/WeKnora/docreader/proto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	knowledgeTypeFile         = "file"
	parseStatusPending        = "pending"
	parseStatusProcessing     = "processing"
	parseStatusCompleted      = "completed"
	parseStatusFailed         = "failed"
	knowledgeUploadTimeout    = 300 * time.Second
	knowledgeDefaultStoreRoot = "data/files"
)

type knowledgeService struct {
	knowledgeRepo     repository.KnowledgeRepository
	knowledgeBaseRepo repository.KnowledgeBaseRepository
	chunkRepo         repository.ChunkRepository
	docReaderClient   *docreaderclient.Client
	imageStorage      ImageStorageService
	cfg               *config.Config
	db                *gorm.DB
	embedderFactory   *llm.EmbedderFactory
}

// NewKnowledgeService 创建知识条目服务
func NewKnowledgeService(
	knowledgeRepo repository.KnowledgeRepository,
	knowledgeBaseRepo repository.KnowledgeBaseRepository,
	chunkRepo repository.ChunkRepository,
	docReaderClient *docreaderclient.Client,
	imageStorage ImageStorageService,
	cfg *config.Config,
	db *gorm.DB,
	embedderFactory *llm.EmbedderFactory,
) KnowledgeService {
	return &knowledgeService{
		knowledgeRepo:     knowledgeRepo,
		knowledgeBaseRepo: knowledgeBaseRepo,
		chunkRepo:         chunkRepo,
		docReaderClient:   docReaderClient,
		imageStorage:      imageStorage,
		cfg:               cfg,
		db:                db,
		embedderFactory:   embedderFactory,
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

	fileBytes, err := fileutil.ReadUploadedFile(fileHeader)
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

	// 异步处理：解析 → 分块 → 向量化（不阻塞 HTTP 响应）
	go s.processUploadAsync(knowledge, fileBytes, fileName, fileType, kb, userID)

	return &dto.KnowledgeFileUploadResponse{
		KnowledgeID:     knowledge.ID,
		KnowledgeBaseID: knowledge.KnowledgeBaseID,
		Title:           knowledge.Title,
		FileType:        knowledge.FileType,
		FilePath:        knowledge.FileURL,
		ParseStatus:     knowledge.ParseStatus,
		ChunkCount:      0,
		MarkdownChars:   0,
	}, nil
}

// processUploadAsync 异步执行解析 → 分块 → 向量化流水线
func (s *knowledgeService) processUploadAsync(knowledge *entity.Knowledge, fileBytes []byte, fileName, fileType string, kb *entity.KnowledgeBase, userID uint) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warnf("[UploadAsync] panic recovered knowledge=%d: %v", knowledge.ID, r)
			_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, fmt.Sprintf("panic: %v", r))
		}
	}()

	ctx := context.Background()

	parseResult, err := s.parseMarkdown(fileBytes, fileName, fileType, kb, knowledge)
	if err != nil {
		logger.Warnf("[UploadAsync] 解析失败 knowledge=%d: %v", knowledge.ID, err)
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return
	}
	if err := s.applyParsedDocumentMetadata(knowledge, parseResult); err != nil {
		logger.Warnf("[UploadAsync] 保存元数据失败 knowledge=%d: %v", knowledge.ID, err)
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return
	}

	chunks, err := s.buildMarkdownChunks(parseResult.MarkdownContent, parseResult.ParserEngine, kb, knowledge)
	if err != nil {
		logger.Warnf("[UploadAsync] 分块失败 knowledge=%d: %v", knowledge.ID, err)
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, err.Error())
		return
	}
	if err := s.chunkRepo.CreateBatch(chunks); err != nil {
		logger.Warnf("[UploadAsync] 保存分块失败 knowledge=%d: %v", knowledge.ID, err)
		_ = s.updateKnowledgeStatus(knowledge, parseStatusFailed, "保存分块失败")
		return
	}

	// 向量化存储（非关键路径，失败不阻塞）
	if err := s.embedChunks(ctx, userID, knowledge, chunks, kb); err != nil {
		logger.Warnf("[UploadAsync] 向量化失败 knowledge=%d: %v", knowledge.ID, err)
	}

	if err := s.updateKnowledgeStatus(knowledge, parseStatusCompleted, ""); err != nil {
		logger.Warnf("[UploadAsync] 更新状态失败 knowledge=%d: %v", knowledge.ID, err)
	}
}

func (s *knowledgeService) PreviewFile(userID uint, knowledgeID uint) (*KnowledgePreviewFile, error) {
	knowledge, err := s.knowledgeRepo.FindByID(knowledgeID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识文件失败", err)
	}
	if knowledge == nil {
		return nil, bizerrors.ErrResourceNotFound
	}

	kb, err := s.knowledgeBaseRepo.FindByUserID(userID, knowledge.KnowledgeBaseID)
	if err != nil {
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "查询知识库失败", err)
	}
	if kb == nil {
		return nil, bizerrors.ErrResourceNotFound
	}

	if knowledge.Type != knowledgeTypeFile && knowledge.Type != entity.KnowledgeTypeFile {
		return nil, bizerrors.New(bizerrors.CodeInvalidParam, "当前知识类型不支持文件预览")
	}

	filePath := strings.TrimSpace(knowledge.FileURL)
	if filePath == "" {
		return nil, bizerrors.New(bizerrors.CodeResourceNotFound, "原始文件不存在")
	}
	if isRemoteResourceURL(filePath) {
		return nil, bizerrors.New(bizerrors.CodeNotImplemented, "远程文件预览暂未实现")
	}

	raw, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, bizerrors.NewWithErr(bizerrors.CodeResourceNotFound, "原始文件不存在", err)
		}
		return nil, bizerrors.NewWithErr(bizerrors.CodeInternalError, "读取原始文件失败", err)
	}

	fileName := strings.TrimSpace(knowledge.FileName)
	if fileName == "" {
		fileName = filepath.Base(filePath)
	}

	contentType := fileutil.DetectPreviewContentType(fileName, raw)
	return &KnowledgePreviewFile{
		FileName:    fileName,
		FileType:    knowledge.FileType,
		ContentType: contentType,
		Content:     raw,
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
	rules := make(map[string]string, len(kb.ChunkingConfig.ParserEngineRules))
	for _, r := range kb.ChunkingConfig.ParserEngineRules {
		for _, ft := range r.FileTypes {
			rules[ft] = r.Engine
		}
	}
	parserEngine := docreader.ResolveParserEngine(rules, fileType)
	if parserEngine != "" {
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
			ContentHash:     fileutil.Sha256Hex(content),
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
	return fileutil.StoreFile(root, fmt.Sprintf("%d", knowledgeBaseID), fileName, fileBytes)
}

func (s *knowledgeService) updateKnowledgeStatus(knowledge *entity.Knowledge, status, errorMessage string) error {
	knowledge.ParseStatus = status
	knowledge.ErrorMessage = errorMessage
	if err := s.knowledgeRepo.Update(knowledge); err != nil {
		return bizerrors.NewWithErr(bizerrors.CodeInternalError, "更新知识状态失败", err)
	}
	return nil
}
