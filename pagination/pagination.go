package pagination

import (
	"fmt"
	"math"
)

type Params struct {
	Limit int `query:"limit" json:"limit,omitempty"`
	Offset int `query:"offset" json:"offset,omitempty"`

	// Page specifies the page number (1-based, used with Size)
	Page int `query:"page" json:"page,omitempty"`
	// Size specifies the number of items per page (used with Page)
	Size int `query:"size" json:"size,omitempty"`

	// Internal field to track which approach was originally used
	usesPageSize bool
}

// Response represents pagination response metadata that can be embedded in response structs.
type Response struct {
	// Total number of items across all pages
	Total int64 `json:"total"`
	// Current page number (1-based)
	Page int `json:"page"`
	// Number of items per page
	Size int `json:"size"`
	// Total number of pages
	Pages int `json:"pages"`
	// Whether there is a next page
	HasNext bool `json:"has_next"`
	// Whether there is a previous page
	HasPrev bool `json:"has_prev"`
}

// Config holds default pagination settings.
type Config struct {
	DefaultLimit int // Default limit when none specified
	MaxLimit     int // Maximum allowed limit
	DefaultSize  int // Default page size when none specified
	MaxSize      int // Maximum allowed page size
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultLimit: 20,
		MaxLimit:     100,
		DefaultSize:  20,
		MaxSize:      100,
	}
}

// hasPageSize returns true if the pagination uses page/size approach
func (p *Params) hasPageSize() bool {
	// If Page or Size is set, it's a page/size approach
	return p.Page > 0 || p.Size > 0
}

// Normalize applies default values and constraints to pagination parameters.
// It handles both limit/offset and page/size approaches.
func (p *Params) Normalize(cfg Config) {
	// Determine which approach was originally used
	// If nothing is set, default to page/size approach
	p.usesPageSize = p.hasPageSize() || (p.Limit == 0 && p.Offset == 0 && p.Page == 0 && p.Size == 0)

	// Handle limit/offset approach
	if p.Limit <= 0 {
		p.Limit = cfg.DefaultLimit
	}
	if p.Limit > cfg.MaxLimit {
		p.Limit = cfg.MaxLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}

	// Handle page/size approach
	if p.Size <= 0 {
		p.Size = cfg.DefaultSize
	}
	if p.Size > cfg.MaxSize {
		p.Size = cfg.MaxSize
	}
	if p.Page <= 0 {
		p.Page = 1
	}

	// If using page/size, convert to limit/offset for consistency
	if p.usesPageSize {
		p.Limit = p.Size
		p.Offset = (p.Page - 1) * p.Size
	} else {
		// If using limit/offset, update page/size for ToPageSize method
		p.Size = p.Limit
		p.Page = (p.Offset / p.Size) + 1
	}
}

// ToLimitOffset returns the limit and offset values.
// This is useful when working with databases that use limit/offset.
func (p *Params) ToLimitOffset() (limit, offset int) {
	return p.Limit, p.Offset
}

// ToPageSize returns the page and size values.
// If page/size weren't originally set, they're calculated from limit/offset.
func (p *Params) ToPageSize() (page, size int) {
	if p.usesPageSize || p.hasPageSize() {
		return p.Page, p.Size
	}

	// Calculate from limit/offset
	size = p.Limit
	if size <= 0 {
		size = 1
	}
	page = (p.Offset / size) + 1
	return page, size
}

// NewResponse creates a pagination response based on the parameters and total count.
func (p *Params) NewResponse(total int64) Response {
	page, size := p.ToPageSize()

	totalPages := int(math.Ceil(float64(total) / float64(size)))
	if totalPages < 1 {
		totalPages = 1
	}

	return Response{
		Total:   total,
		Page:    page,
		Size:    size,
		Pages:   totalPages,
		HasNext: page < totalPages,
		HasPrev: page > 1,
	}
}

// String returns a string representation of the pagination parameters.
func (p *Params) String() string {
	// If Normalize() was called, use the stored preference
	// Otherwise, determine from current field values
	usesPageSizeFormat := p.usesPageSize
	if !p.usesPageSize && p.hasPageSize() && p.Limit == 0 && p.Offset == 0 {
		usesPageSizeFormat = true
	}

	if usesPageSizeFormat {
		return fmt.Sprintf("page=%d size=%d", p.Page, p.Size)
	}
	return fmt.Sprintf("limit=%d offset=%d", p.Limit, p.Offset)
}

// String returns a string representation of the pagination response.
func (r *Response) String() string {
	return fmt.Sprintf("page %d of %d (total: %d, size: %d)", r.Page, r.Pages, r.Total, r.Size)
}
