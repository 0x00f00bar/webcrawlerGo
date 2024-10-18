package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/models"
)

const DriverNameSQLite = "sqlite3"

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

// InitDatabase will create the required tables for the crawlers to use
func (sq SQLiteDB) InitDatabase(ctx context.Context, db *sql.DB) error {
	createURLTableQuery := `CREATE TABLE IF NOT EXISTS urls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT UNIQUE NOT NULL,
	first_encountered DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_checked DATETIME DEFAULT NULL,
	last_saved DATETIME DEFAULT NULL,
	is_monitored BOOLEAN NOT NULL DEFAULT 0,
	version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 0)
	);`
	createPagesTableQuery := `CREATE TABLE IF NOT EXISTS pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url_id INTEGER NOT NULL,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    content TEXT NOT NULL,
    FOREIGN KEY (url_id) REFERENCES urls (id) ON DELETE CASCADE
	);`
	createPagesURLIDIndex := `CREATE INDEX IF NOT EXISTS idx_page_url_id ON pages(url_id);`

	queries := []string{createURLTableQuery, createPagesTableQuery, createPagesURLIDIndex}

	for _, query := range queries {
		timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := db.ExecContext(timeOutCtx, query)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExecWALCheckpoint will initiate checkpoint in the WAL journal
func ExecWALCheckpoint(driverName string, dbWriter *sql.DB) error {
	if driverName == DriverNameSQLite {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := dbWriter.ExecContext(ctx, "PRAGMA wal_checkpoint(FULL);")
		if err != nil {
			return err
		}
	}
	return nil
}
