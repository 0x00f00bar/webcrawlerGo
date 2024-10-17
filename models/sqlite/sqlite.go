package sqlite

import (
	"database/sql"
	"strings"

	"github.com/0x00f00bar/webcrawlerGo/models"
)

// sqliteConnections holds seperate connections to multiple readers
// and a single writer, as SQLite supports only one writer
// we will enable WAL journal mode for multiple concurrent readers
// along with a writer
type sqliteConnections struct {
	readers *sql.DB
	writer  *sql.DB
}

type SQLiteDB struct {
	URLModel  *urlDB
	PageModel *pageDB
}

// NewSQLiteDB returns new instance of SQLiteDB with URL and Pages models
func NewSQLiteDB(dbReader *sql.DB, dbWriter *sql.DB) *SQLiteDB {
	sqliteConns := &sqliteConnections{
		readers: dbReader,
		writer:  dbWriter,
	}
	return &SQLiteDB{
		URLModel:  newUrlDB(sqliteConns),
		PageModel: newPageDB(sqliteConns),
	}
}

// makeSQLiteQuery converts general SQL query to
// PgSQL dialect query by replacing replaceStr in query
func makeSQLiteQuery(query string) string {
	query = strings.ReplaceAll(query, models.QueryArgStr, "?")
	return query
}
