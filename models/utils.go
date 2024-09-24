package models

import "github.com/0x00f00bar/web-crawler/common"

func ValidOrderBy(orderBy string, validFields []string) bool {
	return common.ValuePresent(orderBy, validFields)
}
