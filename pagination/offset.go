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
	PageNumber int   `json:"page_number"`
	PageSize   int   `json:"page_size"`
	Count      int64 `json:"count"`
	Content    []T   `json:"content"`
}

// NewResponse creates paginated response from items and total count.
func NewResponse[T any](items []T, totalCount int64, req Request) Response[T] {
	return Response[T]{
		PageNumber: req.PageNumber,
		PageSize:   req.PageSize,
		Count:      totalCount,
		Content:    items,
	}
}
