package psql

import (
	"database/sql"

	"github.com/0x00f00bar/webcrawlerGo/models"
)

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
	query := makePgSQLQuery(models.QueryGetURLById)

	return models.URLGetById(id, query, u.DB)
}

// GetByURL fetches a row from urls table by url string
func (u urlDB) GetByURL(urlStr string) (*models.URL, error) {
	query := makePgSQLQuery(models.QueryGetURLByURL)

	return models.URLGetByURL(urlStr, query, u.DB)
}

// Insert writes a url to urls table
func (u urlDB) Insert(m *models.URL) error {
	query := makePgSQLQuery(models.QueryInsertURL)

	return models.URLInsert(m, query, u.DB)
}

// Update updates a urls table row with provided values.
// Optimistic lockin enabled: if version change detected
// return ErrEditConflict
func (u urlDB) Update(m *models.URL) error {
	query := makePgSQLQuery(models.QueryUpdateURL)

	return models.URLUpdate(m, query, u.DB)
}

// Delete url row by id
func (u urlDB) Delete(id int) error {
	query := makePgSQLQuery(models.QueryDeleteURL)

	return models.URLDelete(id, query, u.DB)
}

// GetAll fetches all rows from urls table in orderBy order
func (u urlDB) GetAll(uf models.URLFilter, cf models.CommonFilters) ([]*models.URL, models.Metadata, error) {
	return models.URLGetAll(uf, cf, u.DB, makePgSQLQuery)
}

// GetAll fetches all rows where is_monitored is true from urls table in orderBy order
func (u urlDB) GetAllMonitored(cf models.CommonFilters) ([]*models.URL, models.Metadata, error) {
	uf := models.URLFilter{
		IsMonitored:        true,
		IsMonitoredPresent: true,
		IsAlive:            true,
		IsAlivePresent:     true,
	}

	return models.URLGetAll(uf, cf, u.DB, makePgSQLQuery)
}
