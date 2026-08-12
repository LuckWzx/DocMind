package files

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// localPrefix 本地存储协议前缀：local://<相对路径>，相对路径以 storage root 为基准
const localPrefix = "local://"

// defaultStorageRoot 本地存储默认根目录（与 knowledgeService.knowledgeDefaultStoreRoot 一致）
const defaultStorageRoot = "data/files"

// Controller 本地文件代理：渲染 Markdown 图片用（如沙箱图表 local://sandbox/xxx.png）。
// 前端 fetch 时携带 Authorization header（见 web/src/utils/security.ts），
// 拿到 blob URL 后再交给 <img>，避免 token 出现在 URL 中。
type Controller struct {
	root string // 本地存储根目录（config.storage.local_root，默认 data/files）
}

// NewController 创建文件代理控制器；root 为空时使用默认 data/files
func NewController(root string) *Controller {
	if root == "" {
		root = defaultStorageRoot
	}
	return &Controller{root: root}
}

// Serve GET /files?file_path=local://sandbox/xxx.png
// 仅支持 local:// 协议；路径规范化并限制在存储根目录内，防路径穿越
func (c *Controller) Serve(g *gin.Context) {
	filePath := strings.TrimSpace(g.Query("file_path"))
	if !strings.HasPrefix(filePath, localPrefix) {
		g.Status(404)
		return
	}
	rel := strings.TrimPrefix(filePath, localPrefix)
	if rel == "" || strings.Contains(rel, "..") {
		g.Status(404)
		return
	}

	root := filepath.Clean(c.root)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	// 防路径穿越：清理后必须仍在 root 内
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		g.Status(404)
		return
	}
	// 文件不存在时 c.File 返回 404
	g.File(abs)
}
