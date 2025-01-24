package psql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/models"
)

const DriverNamePgSQL = "postgres"

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

// InitDatabase will create the required tables for the crawlers to use
func (pq PsqlDB) InitDatabase(ctx context.Context, db *sql.DB) error {
	createURLTableQuery := `CREATE TABLE IF NOT EXISTS urls (
    id bigserial PRIMARY KEY,
    url TEXT UNIQUE NOT NULL,  -- Using TEXT instead of citext
    first_encountered timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    last_checked timestamp(0) with time zone DEFAULT NULL,
    last_saved timestamp(0) with time zone DEFAULT NULL,
    is_monitored BOOLEAN NOT NULL DEFAULT false,
    version integer NOT NULL DEFAULT 1 CHECK (version >= 0)
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_lower_url ON urls (LOWER(url));`
	createPagesTableQuery := `CREATE TABLE IF NOT EXISTS pages(
    id bigserial PRIMARY KEY,
    url_id bigint NOT NULL REFERENCES urls ON DELETE CASCADE,
    added_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    content text NOT NULL
	);`
	createPagesURLIDIndex := `CREATE INDEX IF NOT EXISTS idx_page_url_id ON pages(url_id);`
	alterURLAddIsAlive := `ALTER TABLE urls
ADD COLUMN IF NOT EXISTS is_alive BOOLEAN DEFAULT TRUE;`

	queries := []string{
		createURLTableQuery,
		createPagesTableQuery,
		createPagesURLIDIndex,
		alterURLAddIsAlive,
	}

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
