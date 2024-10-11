package models

import (
	"github.com/0x00f00bar/web-crawler/internal"
)

// ValidOrderBy tells if the orderBy is present in validFields
func ValidOrderBy(orderBy string, validFields []string) bool {
	return internal.ValuePresent(orderBy, validFields)
}
