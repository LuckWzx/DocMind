package response

// KnowledgeFileUploadResponse 文件上传并解析响应
type KnowledgeFileUploadResponse struct {
	KnowledgeID     uint   `json:"knowledge_id"`
	KnowledgeBaseID uint   `json:"knowledge_base_id"`
	Title           string `json:"title"`
	FileType        string `json:"file_type"`
	FilePath        string `json:"file_path"`
	ParseStatus     string `json:"parse_status"`
	ChunkCount      int    `json:"chunk_count"`
	MarkdownChars   int    `json:"markdown_chars"`
}
