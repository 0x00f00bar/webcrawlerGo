package models

import (
	"time"
)

type Page struct {
	ID      uint
	URL     string
	AddedAt time.Time
	Content string
}
