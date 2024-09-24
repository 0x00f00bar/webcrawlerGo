package models

import (
	"github.com/0x00f00bar/web-crawler/internal"
)

func ValidOrderBy(orderBy string, validFields []string) bool {
	return internal.ValuePresent(orderBy, validFields)
}
