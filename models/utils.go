package models

import (
	"strings"

	"github.com/0x00f00bar/webcrawlerGo/internal"
)

// ValidOrderBy tells if the orderBy is present in validFields
func ValidOrderBy(orderBy string, validFields []string) bool {
	return internal.ValuePresent(orderBy, validFields)
}

// GetOrderByQuery returns ORDER BY sub query as per CommonFilters
func GetOrderByQuery(f *CommonFilters) (string, error) {
	if f.Sort == "" {
		f.Sort = "id"
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 10
	}
	ascOrder := true
	if strings.HasPrefix(f.Sort, "-") {
		ascOrder = false
		f.Sort = strings.TrimPrefix(f.Sort, "-")
	}
	if !ValidOrderBy(f.Sort, f.SortSafeList) {
		return "", ErrInvalidOrderBy
	}
	query := " ORDER BY " + f.Sort

	if !ascOrder {
		query += " DESC"
	}
	return query, nil
}
