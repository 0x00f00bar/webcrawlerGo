package models

import (
	"time"
)

type URLs struct {
	URL              string
	FirstEncountered time.Time
	LastChecked      time.Time
	LastDownloaded   time.Time
	IsMonitored      bool
	Version          int
}
