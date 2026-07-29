package request

type TagListRequest struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Keyword  string `form:"keyword"`
}

type CreateTagRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=255"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

type UpdateTagRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder *int   `json:"sort_order"`
}
