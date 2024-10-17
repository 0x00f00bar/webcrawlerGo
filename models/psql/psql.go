package psql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/0x00f00bar/webcrawlerGo/models"
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

// makePgSQLQuery converts general SQL query to
// PgSQL dialect query by replacing replaceStr in query
func makePgSQLQuery(query string) string {
	argCount := strings.Count(query, models.QueryArgStr)
	for i := range argCount {
		argStr := fmt.Sprintf("$%d", i+1)
		query = strings.Replace(query, models.QueryArgStr, argStr, 1)
	}
	return query
}
