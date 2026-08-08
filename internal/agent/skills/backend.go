package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk/filesystem"
)

// LocalBackend 基于本地文件系统的 filesystem.Backend 实现（只读）
// 供 skill.NewBackendFromFilesystem 使用：skill middleware 仅依赖 Read / GlobInfo，
// 其余方法（LsInfo / GrepRaw / Write / Edit）返回不支持错误
type LocalBackend struct{}

// NewLocalBackend 创建本地文件系统 Backend
func NewLocalBackend() *LocalBackend {
	return &LocalBackend{}
}

// Read 读取文件内容（支持按行 Offset/Limit 切片）
func (b *LocalBackend) Read(_ context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	data, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	content := string(data)
	if req.Offset > 1 || req.Limit > 0 {
		lines := strings.Split(content, "\n")
		start := req.Offset - 1
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
		end := len(lines)
		if req.Limit > 0 && start+req.Limit < end {
			end = start + req.Limit
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return &filesystem.FileContent{Content: content}, nil
}

// GlobInfo 按 glob 模式匹配文件（返回绝对路径，供 skill backend 直接使用）
func (b *LocalBackend) GlobInfo(_ context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	pattern := req.Pattern
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(req.Path, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob 匹配失败: %w", err)
	}
	infos := make([]filesystem.FileInfo, 0, len(matches))
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil {
			continue // 文件被删除等竞态场景直接跳过
		}
		// 必须返回绝对路径：官方 skill backend 仅对相对路径再次 Join(baseDir, path)
		abs, err := filepath.Abs(m)
		if err != nil {
			continue
		}
		infos = append(infos, filesystem.FileInfo{
			Path:       abs,
			IsDir:      fi.IsDir(),
			Size:       fi.Size(),
			ModifiedAt: fi.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return infos, nil
}

// LsInfo 未支持
func (b *LocalBackend) LsInfo(context.Context, *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("LocalBackend 不支持 LsInfo")
}

// GrepRaw 未支持
func (b *LocalBackend) GrepRaw(context.Context, *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	return nil, fmt.Errorf("LocalBackend 不支持 GrepRaw")
}

// Write 未支持（技能目录为只读资源）
func (b *LocalBackend) Write(context.Context, *filesystem.WriteRequest) error {
	return fmt.Errorf("LocalBackend 不支持 Write")
}

// Edit 未支持
func (b *LocalBackend) Edit(context.Context, *filesystem.EditRequest) error {
	return fmt.Errorf("LocalBackend 不支持 Edit")
}
