package models

import (
	"time"
)

// Page type holds the information of URL content
// saved in model
type Page struct {
	ID      uint
	URLID   uint
	AddedAt time.Time
	Content string
}

// NewPage returns new Page type with AddedAt set to current time.
func NewPage(urlId uint, content string) *Page {
	return &Page{
		URLID:   urlId,
		AddedAt: time.Now(),
		Content: content,
	}
}
