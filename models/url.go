package models

import (
	"time"
)

type URL struct {
	ID               uint
	URL              string
	FirstEncountered time.Time
	LastChecked      time.Time
	LastDownloaded   time.Time
	IsMonitored      bool
	Version          uint
}
