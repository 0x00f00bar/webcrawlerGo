package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/0x00f00bar/web-crawler/models"
)

var validURLColumns = []string{"id", "url", "first_encountered", "last_checked",
	"last_saved", "is_monitored", "version"}

// urlDB is used to implement URLModel interface
type urlDB struct {
	DB *sql.DB
}

// newUrlDB returns *urlDB which implements URLModel interface
func newUrlDB(db *sql.DB) *urlDB {
	return &urlDB{
		DB: db,
	}
}

// GetById fetches a row from urls table by id
func (u urlDB) GetById(id int) (*models.URL, error) {

	if id < 1 {
		return nil, models.ErrRecordNotFound
	}

	query := `
	SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, version
	FROM urls
	WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var url models.URL

	err := u.DB.QueryRowContext(ctx, query, id).Scan(
		&url.ID,
		&url.FirstEncountered,
		&url.LastChecked,
		&url.LastSaved,
		&url.IsMonitored,
		&url.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, models.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &url, nil
}

// GetByURL fetches a row from urls table by url string
func (u urlDB) GetByURL(urlStr string) (*models.URL, error) {
	if urlStr == "" {
		return nil, models.ErrNullURL
	}

	query := `
	SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, version
	FROM urls
	WHERE url = $1`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var url models.URL

	err := u.DB.QueryRowContext(ctx, query, urlStr).Scan(
		&url.ID,
		&url.FirstEncountered,
		&url.LastChecked,
		&url.LastSaved,
		&url.IsMonitored,
		&url.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, models.ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &url, nil
}

// Insert writes a url to urls table
func (u urlDB) Insert(m *models.URL) error {
	query := `
	INSERT INTO urls (url, last_checked, last_saved, is_monitored)
	VALUES ($1, $2, $3, $4)
	RETURNING id, first_encountered, version`

	args := []interface{}{m.URL, m.LastChecked, m.LastSaved, m.IsMonitored}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	return u.DB.QueryRowContext(ctx, query, args...).Scan(&m.ID, &m.FirstEncountered, &m.Version)
}

// Update updates a urls table row with provided values.
// Optimistic lockin enabled: if version change detected
// return ErrEditConflict
func (u urlDB) Update(m *models.URL) error {
	query := `
	UPDATE urls
	SET last_checked = $1, last_saved = $2, is_monitored = $3, version = version + 1
	WHERE id = $4 AND version = $5
	RETURNING version`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	args := []interface{}{m.LastChecked, m.LastSaved, m.IsMonitored, m.ID, m.Version}

	err := u.DB.QueryRowContext(ctx, query, args...).Scan(&m.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return models.ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

// Delete url row by id
func (u urlDB) Delete(id int) error {
	if id < 1 {
		return models.ErrRecordNotFound
	}

	query := `
	DELETE from urls
	WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	result, err := u.DB.ExecContext(ctx, query, id)
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

// GetAll fetches all rows from urls table in orderBy order
func (u urlDB) GetAll(orderBy string) ([]*models.URL, error) {

	if !models.ValidOrderBy(orderBy, validURLColumns) {
		return nil, fmt.Errorf("%w : %s", models.ErrInvalidOrderBy, orderBy)
	}

	query := `
	SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, version
	FROM urls
	ORDER BY $1`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	rows, err := u.DB.QueryContext(ctx, query, orderBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := []*models.URL{}

	for rows.Next() {

		var url models.URL

		err := rows.Scan(
			&url.ID,
			&url.URL,
			&url.FirstEncountered,
			&url.LastChecked,
			&url.LastSaved,
			&url.IsMonitored,
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

// GetAll fetches all rows where is_monitored is true from urls table in orderBy order
func (u urlDB) GetAllMonitored(orderBy string) ([]*models.URL, error) {

	if !models.ValidOrderBy(orderBy, validURLColumns) {
		return nil, fmt.Errorf("%w : %s", models.ErrInvalidOrderBy, orderBy)
	}

	query := `
	SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, version
	FROM urls
	WHERE is_monitored = true
	ORDER BY $1`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	rows, err := u.DB.QueryContext(ctx, query, orderBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := []*models.URL{}

	for rows.Next() {

		var url models.URL

		err := rows.Scan(
			&url.ID,
			&url.URL,
			&url.FirstEncountered,
			&url.LastChecked,
			&url.LastSaved,
			&url.IsMonitored,
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
