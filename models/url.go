package models

import (
	"time"
)

// URL type holds the information of URL
// saved in model
type URL struct {
	ID               uint
	URL              string
	FirstEncountered time.Time
	LastChecked      time.Time
	LastSaved        time.Time
	IsMonitored      bool
	Version          uint
}

// NewURL returns new URL type with FirstEncountered set to time.Now
func NewURL(url string, lastChecked, lastSaved time.Time, isMonitored bool) *URL {
	return &URL{
		URL:              url,
		FirstEncountered: time.Now(),
		LastChecked:      lastChecked,
		LastSaved:        lastSaved,
		IsMonitored:      isMonitored,
	}
}
