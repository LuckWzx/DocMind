package fileutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ReadUploadedFile 从 multipart FileHeader 读取全量字节
func ReadUploadedFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// StoreFile 将文件字节写入磁盘，存储到 baseDir/<subDir>/<uuid><ext>
// 返回写入后的绝对/相对路径
func StoreFile(baseDir string, subDir string, fileName string, fileBytes []byte) (string, error) {
	extension := filepath.Ext(fileName)
	storedName := uuid.NewString() + extension
	targetPath := filepath.Join(baseDir, subDir, storedName)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, fileBytes, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

// DetectPreviewContentType 根据文件名扩展名和原始字节推断 MIME 类型
func DetectPreviewContentType(fileName string, raw []byte) string {
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
	case ".txt", ".json", ".xml", ".html", ".css", ".js", ".ts", ".py", ".java", ".go", ".cpp",
		".c", ".h", ".sh", ".yaml", ".yml", ".ini", ".conf", ".log", ".sql", ".rs", ".rb",
		".php", ".swift", ".kt", ".scala", ".r", ".lua", ".pl", ".toml":
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

// Sha256Hex 计算输入字符串的 SHA256 十六进制摘要
func Sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
