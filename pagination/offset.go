package pagination

type Request struct {
	PageNumber int `query:"page_number"`
	PageSize   int `query:"page_size"`
}

// Normalize applies defaults and constraints.
func (r *Request) Normalize(opts ...Option) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	if r.PageNumber <= 0 {
		r.PageNumber = 1
	}
	if r.PageSize <= 0 {
		r.PageSize = defaultPageSize
	}
	if r.PageSize > o.MaxPageSize {
		r.PageSize = o.MaxPageSize
	}
}

// Offset returns the offset value.
func (r *Request) Offset() int {
	return (r.PageNumber - 1) * r.PageSize
}

// Limit returns the limit value.
func (r *Request) Limit() int {
	return r.PageSize
}

type Response[T any] struct {
	PageNumber  int   `json:"page_number"`
	PageSize    int   `json:"page_size"`
	PageCount   int   `json:"page_count"`
	TotalCount  int64 `json:"total_count"`
	PageContent []T   `json:"page_content"`
}

// NewResponse creates paginated response from items and total count.
func NewResponse[T any](items []T, totalCount int64, req Request) Response[T] {
	pageCount := int(totalCount) / req.PageSize
	if int(totalCount)%req.PageSize > 0 {
		pageCount++
	}

	return Response[T]{
		PageNumber:  req.PageNumber,
		PageSize:    req.PageSize,
		PageCount:   pageCount,
		TotalCount:  totalCount,
		PageContent: items,
	}
}
