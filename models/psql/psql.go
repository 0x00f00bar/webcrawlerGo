package psql

import (
	"database/sql"
)

type PsqlDB struct {
	URLModel  *urlDB
	PageModel *pageDB
}

func NewPsqlDB(db *sql.DB) *PsqlDB {
	return &PsqlDB{
		URLModel:  newUrlDB(db),
		PageModel: newPageDB(db),
	}
}
