package models

import (
	"time"
)

type Page struct {
	ID      uint
	URLID   uint
	AddedAt time.Time
	Content string
}
