package request

type KnowledgeListRequest struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
	TagIDs      string `form:"tag_ids"`
	Keyword     string `form:"keyword"`
	FileType    string `form:"file_type"`
	ParseStatus string `form:"parse_status"`
	Source      string `form:"source"`
}

type ReparseKnowledgeRequest struct {
	ProcessConfig string `json:"process_config"`
}

type KnowledgeTagBatchUpdateRequest struct {
	Updates map[uint][]uint `json:"updates"`
}
