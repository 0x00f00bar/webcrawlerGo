package psql

import (
	"database/sql"
)

type PsqlDB struct {
	URLModel  *urlDB
	PageModel *pageDB
}

// NewPsqlDB returns new instance of PostgreSQL with URL and Pages models
func NewPsqlDB(db *sql.DB) *PsqlDB {
	return &PsqlDB{
		URLModel:  newUrlDB(db),
		PageModel: newPageDB(db),
	}
}
