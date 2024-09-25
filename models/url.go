package models

import (
	"time"
)

type URL struct {
	ID               uint
	URL              string
	FirstEncountered time.Time
	LastChecked      time.Time
	LastSaved        time.Time
	IsMonitored      bool
	Version          uint
}

func NewURL(url string, lastChecked, lastSaved time.Time, isMonitored bool) *URL {
	return &URL{
		URL:              url,
		FirstEncountered: time.Now(),
		LastChecked:      lastChecked,
		LastSaved:        lastSaved,
		IsMonitored:      isMonitored,
	}
}
