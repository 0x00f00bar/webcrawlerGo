package models

func ValidOrderBy(orderBy string, validFields []string) bool {
	for _, s := range validFields {
		if s == orderBy {
			return true
		}
	}
	return false
}
