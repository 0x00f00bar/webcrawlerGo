package sqlite

import (
	"github.com/0x00f00bar/webcrawlerGo/models"
)

// urlDB is used to implement URLModel interface
type urlDB struct {
	DB *sqliteConnections
}

// newUrlDB returns *urlDB which implements URLModel interface
func newUrlDB(db *sqliteConnections) *urlDB {
	return &urlDB{
		DB: db,
	}
}

// GetById fetches a row from urls table by id
func (u urlDB) GetById(id int) (*models.URL, error) {
	query := makeSQLiteQuery(models.QueryGetURLById)

	return models.URLGetById(id, query, u.DB.readers)
}

// GetByURL fetches a row from urls table by url string
func (u urlDB) GetByURL(urlStr string) (*models.URL, error) {
	query := makeSQLiteQuery(models.QueryGetURLByURL)

	return models.URLGetByURL(urlStr, query, u.DB.readers)
}

// Insert writes a url to urls table
func (u urlDB) Insert(url *models.URL) error {
	query := makeSQLiteQuery(models.QueryInsertURL)

	return models.URLInsert(url, query, u.DB.writer)
}

// Update updates a url with provided values.
// Optimistic locking enabled: if version change detected
// return ErrEditConflict
func (u urlDB) Update(url *models.URL) error {
	query := makeSQLiteQuery(models.QueryUpdateURL)

	return models.URLUpdate(url, query, u.DB.writer)
}

// Delete url row by id
func (u urlDB) Delete(id int) error {
	query := makeSQLiteQuery(models.QueryDeleteURL)

	return models.URLDelete(id, query, u.DB.writer)
}

// GetAll fetches all rows from urls table in orderBy order
func (u urlDB) GetAll(orderBy string) ([]*models.URL, error) {
	query := makeSQLiteQuery(models.QueryGetAllURL)

	return models.URLGetAll(orderBy, query, u.DB.readers)
}

// GetAll fetches all rows where is_monitored is true from urls table in orderBy order
func (u urlDB) GetAllMonitored(orderBy string) ([]*models.URL, error) {
	query := makeSQLiteQuery(models.QueryGetAllMonitoredURL)

	return models.URLGetAll(orderBy, query, u.DB.readers)
}
