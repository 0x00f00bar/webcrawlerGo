package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"
)

var PageColumns = []string{"id", "url_id", "added_at", "content"}

// Queries related to pages table
const (
	QuerySelectPage          = "SELECT id, url_id, added_at, content FROM pages"
	QueryGetPageById         = QuerySelectPage + " WHERE id = __ARG__"
	QueryGetAllPageByURL     = QuerySelectPage + " WHERE url_id = __ARG__ ORDER BY __ARG__"
	QueryInsertPage          = `INSERT INTO pages (url_id, content) VALUES (__ARG__, __ARG__) RETURNING id, added_at`
	QueryDeletePage          = `DELETE from pages WHERE id = __ARG__`
	QueryGetLatestPagesCount = `WITH LatestPages AS (
		SELECT u.url, p.id, p.added_at,
			ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY p.added_at DESC) AS rn
		FROM pages p
		JOIN urls u ON p.url_id = u.id
		WHERE u.is_monitored=true AND u.url LIKE __ARG__ || '%'
		AND u.url LIKE '%' || __ARG__ || '%'
		AND p.added_at <= __ARG__
	)
	SELECT COUNT(*)
	FROM LatestPages
	WHERE rn = 1`
	QueryGetLatestPagesPaginated = `WITH LatestPages AS (
		SELECT u.url, p.added_at, p.content,
			ROW_NUMBER() OVER (PARTITION BY u.id ORDER BY p.added_at DESC) AS rn
		FROM pages p
		JOIN urls u ON p.url_id = u.id
		WHERE u.is_monitored=true AND u.url LIKE __ARG__ || '%'
		AND u.url LIKE '%' || __ARG__ || '%'
		AND p.added_at <= __ARG__
	)
	SELECT *
	FROM LatestPages
	WHERE rn = 1
	LIMIT __ARG__ OFFSET (__ARG__ - 1) * __ARG__;`
)

// Page type holds the information of URL content
// saved in model
type Page struct {
	ID      uint
	URLID   uint
	AddedAt time.Time
	Content string
}

// PageContent type contains feilds required for
// saving page contents to disk
type PageContent struct {
	URL     string
	AddedAt time.Time
	Content string
	rn      int // row number from query
}

// NewPage returns new Page type with AddedAt set to current time.
func NewPage(urlId uint, content string) *Page {
	return &Page{
		URLID:   urlId,
		AddedAt: time.Now(),
		Content: content,
	}
}

// PageGetById fetches a row from pages table by id
func PageGetById(id int, query string, db *sql.DB) (*Page, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	var page Page

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	err := db.QueryRowContext(ctx, query, id).Scan(
		&page.ID,
		&page.URLID,
		&page.AddedAt,
		&page.Content,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &page, nil
}

// PageGetAllByURL fetches a row from pages table by urlId
// and order by orderBy
func PageGetAllByURL(urlID uint, orderBy string, query string, db *sql.DB) ([]*Page, error) {
	if urlID < 1 {
		return nil, ErrRecordNotFound
	}

	if !ValidOrderBy(orderBy, PageColumns) {
		return nil, fmt.Errorf("%w : %s", ErrInvalidOrderBy, orderBy)
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, urlID, orderBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*Page

	for rows.Next() {
		var page Page

		err = rows.Scan(
			&page.ID,
			&page.URLID,
			&page.AddedAt,
			&page.Content,
		)
		if err != nil {
			return nil, err
		}

		pages = append(pages, &page)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pages, nil
}

// PageInsert writes a page to pages table
func PageInsert(m *Page, query string, db *sql.DB) error {

	args := []interface{}{m.URLID, m.Content}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultDBTimeout)
	defer cancel()

	return db.QueryRowContext(ctx, query, args...).Scan(&m.ID, &m.AddedAt)
}

// Update not required on pages table
// func Update(m *Page) error {
// }

// PageDelete delete page row by id
func PageDelete(id int, query string, db *sql.DB) error {
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

// PageGetLatestPageCount returns count of the latest pages
// filtered by baseurl, markedURL and by cutoff date
func PageGetLatestPageCount(
	ctx context.Context,
	baseurl *url.URL,
	markedURL string,
	cutoffDate time.Time,
	query string,
	db *sql.DB,
) (int, error) {
	timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var recordCount int

	commonArgs := []interface{}{baseurl.String(), markedURL, cutoffDate}
	// get total records for the marked url
	err := db.QueryRowContext(timeOutCtx, query, commonArgs...).
		Scan(&recordCount)
	if err != nil {
		return 0, err
	}
	return recordCount, nil
}

// PageGetLatestPagesPaginated returns PageContent of the latest pages
// filtered by baseurl, markedURL and by cutoff date
func PageGetLatestPagesPaginated(
	ctx context.Context,
	baseurl *url.URL,
	markedURL string,
	cutoffDate time.Time,
	pageNum int,
	pageSize int,
	query string,
	db *sql.DB,
) ([]*PageContent, error) {
	timeOutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := db.QueryContext(
		timeOutCtx,
		query,
		baseurl.String(),
		markedURL,
		cutoffDate,
		pageSize,
		pageNum,
		pageSize,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pageContents []*PageContent

	for rows.Next() {
		var pageContent PageContent

		err = rows.Scan(
			&pageContent.URL,
			&pageContent.AddedAt,
			&pageContent.Content,
			&pageContent.rn,
		)

		if err != nil {
			return nil, err
		}

		pageContents = append(pageContents, &pageContent)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pageContents, nil
}
