package psql

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/0x00f00bar/web-crawler/models"
)

var validPageColumns = []string{"id", "url_id", "added_at", "content"}

// pageDB is used to implement PageModel interface
type pageDB struct {
	DB *sql.DB
}

func newPageDB(db *sql.DB) *pageDB {
	return &pageDB{
		DB: db,
	}
}

// GetById fetches a row from pages table by id
func (p pageDB) GetById(id int) (*models.Page, error) {

	if id < 1 {
		return nil, models.ErrRecordNotFound
	}

	query := `
	SELECT id, url_id, added_at, content
	FROM pages
	WHERE id = $1`

	var page models.Page

	err := p.DB.QueryRow(query, id).Scan(
		&page.ID,
		&page.URLID,
		&page.AddedAt,
		&page.Content,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, models.ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &page, nil
}

// GetAllByURL fetches a row from pages table by urlId
// and order by orderBy
func (p pageDB) GetAllByURL(urlID uint, orderBy string) ([]*models.Page, error) {

	if urlID < 1 {
		return nil, models.ErrRecordNotFound
	}

	if !models.ValidOrderBy(orderBy, validPageColumns) {
		return nil, fmt.Errorf("%w : %s", models.ErrInvalidOrderBy, orderBy)
	}

	query := `
	SELECT id, url_id, added_at, content
	FROM pages
	WHERE url_id = $1
	ORDER BY $2`

	rows, err := p.DB.Query(query, urlID, orderBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pages []*models.Page

	for rows.Next() {
		var page models.Page

		err := rows.Scan(
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

// Insert writes a page to pages table
func (p pageDB) Insert(m *models.Page) error {
	query := `
	INSERT INTO pages (url_id, content)
	VALUES ($1, $2)
	RETURNING id, added_at`

	args := []interface{}{m.URLID, m.Content}

	return p.DB.QueryRow(query, args...).Scan(&m.ID, &m.AddedAt)
}

// Update not required on pages table
// func (p pageDB) Update(m *models.Page) error {
// }

// Delete page row by id
func (p pageDB) Delete(id int) error {
	if id < 1 {
		return models.ErrRecordNotFound
	}

	query := `
	DELETE from pages
	WHERE id = $1`

	result, err := p.DB.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return models.ErrRecordNotFound
	}

	return nil
}
