package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var URLColumns = []string{
	"id", "url", "first_encountered", "last_checked",
	"last_saved", "is_monitored", "is_alive", "version",
}

type URLFilter struct {
	URL                string
	IsMonitored        bool
	IsMonitoredPresent bool
	IsAlive            bool
	IsAlivePresent     bool
}

// Queries related to urls table
const (
	QuerySelectURL   = "SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, is_alive, version FROM urls "
	QueryGetURLById  = QuerySelectURL + "WHERE id = __ARG__"
	QueryGetURLByURL = QuerySelectURL + "WHERE url = __ARG__"
	QueryInsertURL   = `
	INSERT INTO urls (url, last_checked, last_saved, is_monitored)
	VALUES (__ARG__, __ARG__, __ARG__, __ARG__)
	RETURNING id, first_encountered, version`
	QueryUpdateURL = `
	UPDATE urls
	SET last_checked = __ARG__, last_saved = __ARG__, is_monitored = __ARG__, is_alive = __ARG__, version = version + 1
	WHERE id = __ARG__ AND version = __ARG__
	RETURNING version`
	QueryDeleteURL          = `DELETE from urls WHERE id = __ARG__`
	QueryGetAllURL          = QuerySelectURL + "WHERE url LIKE __ARG__ "
	QueryGetAllMonitoredURL = QuerySelectURL + `
	WHERE is_monitored = true AND is_alive = true
	ORDER BY __ARG__`
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
	IsAlive          bool
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
		IsAlive:          true,
	}
}

// URLGetById fetches a row from urls table by id
func URLGetById(id int, query string, db *sql.DB) (*URL, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	var url URL

	err := db.QueryRowContext(ctx, query, id).Scan(
		&url.ID,
		&url.URL,
		&url.FirstEncountered,
		&url.LastChecked,
		&url.LastSaved,
		&url.IsMonitored,
		&url.IsAlive,
		&url.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &url, nil
}

// URLGetByURL fetches a row from urls table by url string
func URLGetByURL(urlStr string, query string, db *sql.DB) (*URL, error) {
	if urlStr == "" {
		return nil, ErrNullURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	var url URL

	err := db.QueryRowContext(ctx, query, urlStr).Scan(
		&url.ID,
		&url.URL,
		&url.FirstEncountered,
		&url.LastChecked,
		&url.LastSaved,
		&url.IsMonitored,
		&url.IsAlive,
		&url.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &url, nil
}

// URLInsert writes a url to urls table
func URLInsert(m *URL, query string, db *sql.DB) error {

	args := []interface{}{m.URL, m.LastChecked, m.LastSaved, m.IsMonitored}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	return db.QueryRowContext(ctx, query, args...).Scan(&m.ID, &m.FirstEncountered, &m.Version)
}

// URLUpdate updates a url with provided values.
// Optimistic locking enabled: if version change detected
// return ErrEditConflict
func URLUpdate(m *URL, query string, db *sql.DB) error {

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	args := []interface{}{m.LastChecked, m.LastSaved, m.IsMonitored, m.IsAlive, m.ID, m.Version}

	err := db.QueryRowContext(ctx, query, args...).Scan(&m.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

// URLDelete url row by id
func URLDelete(id int, query string, db *sql.DB) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	result, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// URLGetAll fetches all rows from urls table as per filters
func URLGetAll(uf URLFilter, cf CommonFilters, query string, db *sql.DB) ([]*URL, error) {

	url := fmt.Sprintf("%%%s%%", uf.URL)
	args := []any{url}

	if uf.IsAlivePresent {
		query += " AND is_alive = __ARG__"
		args = append(args, uf.IsAlive)
	}
	if uf.IsMonitoredPresent {
		query += " AND is_monitored = __ARG__"
		args = append(args, uf.IsMonitored)
	}

	orderBy, err := GetOrderByQuery(&cf)
	if err != nil {
		return nil, err
	}
	query += orderBy

	query += " LIMIT __ARG__ OFFSET __ARG__"
	args = append(args, cf.Limit(), cf.Offset())

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := []*URL{}

	for rows.Next() {

		var url URL

		err = rows.Scan(
			&url.ID,
			&url.URL,
			&url.FirstEncountered,
			&url.LastChecked,
			&url.LastSaved,
			&url.IsMonitored,
			&url.IsAlive,
			&url.Version,
		)
		if err != nil {
			return nil, err
		}

		urls = append(urls, &url)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}
