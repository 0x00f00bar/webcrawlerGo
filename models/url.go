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
