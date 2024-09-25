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

// NewPage returns new Page type with AddedAt set to current time.
// Update fields as required before saving the model.
func NewPage(urlId uint, content string) *Page {
	return &Page{
		URLID:   urlId,
		AddedAt: time.Now(),
		Content: content,
	}
}
