package psql

import (
	"database/sql"

	"github.com/0x00f00bar/web-crawler/models"
)

// pageDB is used to implement PageModel interface
type pageDB struct {
	DB *sql.DB
}

func newPageDB(db *sql.DB) *pageDB {
	return &pageDB{
		DB: db,
	}
}

// methods for Page Model

func (p pageDB) GetById(id int) (*models.Page, error) {

	if id < 1 {
		return nil, models.ErrRecordNotFound
	}

	// query := `
	// SELECT id, url, first_encountered, last_checked, last_saved, is_monitored, version
	// FROM urls
	// WHERE id = $1`

	var page models.Page

	return &page, nil
}

func (p pageDB) GetByURL(urlStr string) (*models.Page, error) {
	return &models.Page{}, nil
}

func (p pageDB) Insert(m *models.Page) error {
	return nil
}

func (p pageDB) Update(m *models.Page) error {
	return nil
}

func (p pageDB) Delete(id int) error {
	return nil
}
