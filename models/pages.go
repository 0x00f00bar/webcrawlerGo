package models

import (
	"time"
)

type Pages struct {
	URL     string
	AddedAt time.Time
	Content string
}
