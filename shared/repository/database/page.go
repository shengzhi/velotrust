package database

type Paging interface {
	Limit() int
	Offset() int
}

type Page struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func (p *Page) Offset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit()
	return offset
}

func (p *Page) Limit() int {
	if p.PageSize <= 0 || p.PageSize > 1000 {
		return 50
	}
	return p.PageSize
}
