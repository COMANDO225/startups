package pagination

// Result is a generic paginated result.
type Result[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

// NewResult creates a paginated result.
func NewResult[T any](items []T, total int64, page, limit int) Result[T] {
	if items == nil {
		items = []T{}
	}
	return Result[T]{
		Items: items,
		Total: total,
		Page:  page,
		Limit: limit,
	}
}

// HasMore returns true if there are more pages.
func (r Result[T]) HasMore() bool {
	return int64(r.Page*r.Limit) < r.Total
}

// PageCount returns the total number of pages.
func (r Result[T]) PageCount() int {
	if r.Limit <= 0 {
		return 0
	}
	pages := int(r.Total) / r.Limit
	if int(r.Total)%r.Limit > 0 {
		pages++
	}
	return pages
}

// Params holds pagination input parameters.
type Params struct {
	Page  int `query:"page" validate:"omitempty,min=1"`
	Limit int `query:"limit" validate:"omitempty,min=1,max=100"`
}

// Normalize sets defaults for missing pagination params.
func (p *Params) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
}

// Offset returns the SQL OFFSET value.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}
