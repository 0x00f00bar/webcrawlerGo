package sqlite

import (
	"context"
	"net/url"
	"time"

	"github.com/0x00f00bar/web-crawler/models"
)

// pageDB is used to implement PageModel interface
type pageDB struct {
	DB *sqliteConnections
}

// newUrlDB returns *urlDB which implements URLModel interface
func newPageDB(db *sqliteConnections) *pageDB {
	return &pageDB{
		DB: db,
	}
}

// GetById fetches a row from pages table by id
func (p pageDB) GetById(id int) (*models.Page, error) {
	query := makeSQLiteQuery(models.QueryGetPageById)

	return models.PageGetById(id, query, p.DB.readers)
}

// GetAllByURL fetches a row from pages table by urlId
// and order by orderBy
func (p pageDB) GetAllByURL(urlID uint, orderBy string) ([]*models.Page, error) {
	query := makeSQLiteQuery(models.QueryGetAllPageByURL)

	return models.PageGetAllByURL(urlID, orderBy, query, p.DB.readers)
}

// Insert writes a page to pages table
func (p pageDB) Insert(m *models.Page) error {
	query := makeSQLiteQuery(models.QueryInsertPage)

	return models.PageInsert(m, query, p.DB.writer)
}

// Update not required on pages table
// func (p pageDB) Update(m *models.Page) error {
// }

// Delete page row by id
func (p pageDB) Delete(id int) error {
	query := makeSQLiteQuery(models.QueryDeletePage)

	return models.PageDelete(id, query, p.DB.writer)
}

// GetLatestPageCount returns the number of latest pages
// filtered by baseurl, markedURL and by cutoff date
func (p pageDB) GetLatestPageCount(
	ctx context.Context,
	baseURL *url.URL,
	markedURL string,
	cutoffDate time.Time,
) (int, error) {
	query := makeSQLiteQuery(models.QueryGetLatestPagesCount)

	return models.PageGetLatestPageCount(ctx, baseURL, markedURL, cutoffDate, query, p.DB.readers)
}

// GetLatestPagesPaginated returns PageContent of latest pages
// filtered by baseurl, markedURL and by cutoff date
func (p pageDB) GetLatestPagesPaginated(
	ctx context.Context,
	baseURL *url.URL,
	markedURL string,
	cutoffDate time.Time,
	pageNum int,
	pageSize int,
) ([]*models.PageContent, error) {
	query := makeSQLiteQuery(models.QueryGetLatestPagesPaginated)

	return models.PageGetLatestPagesPaginated(
		ctx,
		baseURL,
		markedURL,
		cutoffDate,
		pageNum,
		pageSize,
		query,
		p.DB.readers,
	)
}
