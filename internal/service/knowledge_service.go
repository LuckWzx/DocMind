package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
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
	"docmind/pkg/logger"

	"github.com/Tencent/WeKnora/docreader/proto"
	"github.com/google/uuid"
	pgvector "github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	db                *gorm.DB
	embedderFactory   *EmbedderFactory
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
	embedderFactory *EmbedderFactory,
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

	// 向量化存储（非关键路径，失败不阻塞上传）
	if err := s.embedChunks(context.Background(), userID, knowledge, chunks, kb); err != nil {
		logger.Warnf("[UploadFile] 向量化失败: %v", err)
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

// embedChunks 将分块内容向量化并写入 chunk_vectors 表
func (s *knowledgeService) embedChunks(ctx context.Context, userID uint, knowledge *entity.Knowledge, chunks []*entity.Chunk, kb *entity.KnowledgeBase) error {
	if s.embedderFactory == nil || s.db == nil {
		logger.Warnf("[embedChunks] embedderFactory 或 db 未初始化，跳过向量化")
		return nil
	}
	if kb.EmbeddingModelID == "" {
		logger.Warnf("[embedChunks] 知识库 %d 未配置 EmbeddingModelID，跳过向量化", kb.ID)
		return nil
	}

	embedder, err := s.embedderFactory.CreateEmbedder(ctx, kb.EmbeddingModelID)
	if err != nil {
		return fmt.Errorf("创建 Embedder 失败 (modelID=%s): %w", kb.EmbeddingModelID, err)
	}

	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	vectors2D, err := embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return fmt.Errorf("向量化失败: %w", err)
	}

	vectorStoreID := kb.VectorStoreID
	if vectorStoreID == nil {
		var defaultStore entity.VectorStore
		if err := s.db.Where("status = ?", entity.VectorStoreStatusActive).First(&defaultStore).Error; err != nil {
			logger.Warnf("[embedChunks] 知识库 %d 未配置 VectorStoreID 且未找到可用向量存储，跳过向量化", kb.ID)
			return nil
		}
		id := defaultStore.ID
		vectorStoreID = &id
	}

	for i, chunk := range chunks {
		if i >= len(vectors2D) {
			break
		}
		vec32 := make([]float32, len(vectors2D[i]))
		for j, v := range vectors2D[i] {
			vec32[j] = float32(v)
		}
		record := &entity.ChunkVector{
			UserID:          userID,
			VectorStoreID:   *vectorStoreID,
			KnowledgeBaseID: knowledge.KnowledgeBaseID,
			KnowledgeID:     knowledge.ID,
			ChunkID:         chunk.ID,
			Embedding:       pgvector.NewVector(vec32),
			ContentHash:     chunk.ContentHash,
			IsEnabled:       true,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chunk_id"}},
			UpdateAll: true,
		}).Create(record).Error; err != nil {
			return fmt.Errorf("保存向量失败: %w", err)
		}
	}
	return nil
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

	contentType := detectPreviewContentType(fileName, raw)
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
	if parserEngine := resolveParserEngine(kb.ChunkingConfig.ParserEngineRules, fileType); parserEngine != "" {
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

func resolveParserEngine(rules []entity.ParserEngineRule, fileType string) string {
	if parserEngine := selectParserEngine(rules, fileType); parserEngine != "" {
		return parserEngine
	}
	return defaultParserEngine(fileType)
}

func defaultParserEngine(fileType string) string {
	switch strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".") {
	case "ppt", "pptx", "csv":
		return "markitdown"
	default:
		return ""
	}
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

func detectPreviewContentType(fileName string, raw []byte) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	if ext != "" {
		if contentType := previewContentTypeByExt(ext); contentType != "" {
			return contentType
		}
		if contentType := strings.TrimSpace(mime.TypeByExtension(ext)); contentType != "" {
			return contentType
		}
	}

	if len(raw) > 0 {
		return http.DetectContentType(raw)
	}
	return "application/octet-stream"
}

func previewContentTypeByExt(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".doc":
		return "application/msword"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".csv":
		return "text/csv; charset=utf-8"
	case ".md", ".markdown":
		return "text/markdown; charset=utf-8"
	case ".txt", ".json", ".xml", ".html", ".css", ".js", ".ts", ".py", ".java", ".go", ".cpp", ".c", ".h", ".sh", ".yaml", ".yml", ".ini", ".conf", ".log", ".sql", ".rs", ".rb", ".php", ".swift", ".kt", ".scala", ".r", ".lua", ".pl", ".toml":
		return "text/plain; charset=utf-8"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".webp":
		return "image/webp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	default:
		return ""
	}
}
