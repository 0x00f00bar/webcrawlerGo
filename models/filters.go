package models

import (
	"math"

	"github.com/0x00f00bar/webcrawlerGo/internal"
)

type CommonFilters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func (c CommonFilters) Limit() int {
	return c.PageSize
}

func (c CommonFilters) Offset() int {
	return (c.Page - 1) * c.PageSize
}

func ValidateCommonFilters(v *internal.Validator, f CommonFilters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 1_000_000, "page", "cannot be greater than 1 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	v.Check(internal.PermittedValue(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}
