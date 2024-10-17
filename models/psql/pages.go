package psql

import (
	"context"
	"database/sql"
	"net/url"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/models"
)

// pageDB is used to implement PageModel interface
type pageDB struct {
	DB *sql.DB
}

// newPageDB returns *pageDB which implements PageModel interface
func newPageDB(db *sql.DB) *pageDB {
	return &pageDB{
		DB: db,
	}
}

// GetById fetches a row from pages table by id
func (p pageDB) GetById(id int) (*models.Page, error) {
	query := makePgSQLQuery(models.QueryGetPageById)

	return models.PageGetById(id, query, p.DB)
}

// GetAllByURL fetches a row from pages table by urlId
// and order by orderBy
func (p pageDB) GetAllByURL(urlID uint, orderBy string) ([]*models.Page, error) {
	query := makePgSQLQuery(models.QueryGetAllPageByURL)

	return models.PageGetAllByURL(urlID, orderBy, query, p.DB)
}

// Insert writes a page to pages table
func (p pageDB) Insert(m *models.Page) error {
	query := makePgSQLQuery(models.QueryInsertPage)

	return models.PageInsert(m, query, p.DB)
}

// Update not required on pages table
// func (p pageDB) Update(m *models.Page) error {
// }

// Delete page row by id
func (p pageDB) Delete(id int) error {
	query := makePgSQLQuery(models.QueryDeletePage)

	return models.PageDelete(id, query, p.DB)
}

// GetLatestPageCount returns the number of latest pages
// filtered by baseurl, markedURL and by cutoff date
func (p pageDB) GetLatestPageCount(
	ctx context.Context,
	baseURL *url.URL,
	markedURL string,
	cutoffDate time.Time,
) (int, error) {
	query := makePgSQLQuery(models.QueryGetLatestPagesCount)

	return models.PageGetLatestPageCount(ctx, baseURL, markedURL, cutoffDate, query, p.DB)
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
	query := makePgSQLQuery(models.QueryGetLatestPagesPaginated)

	return models.PageGetLatestPagesPaginated(
		ctx,
		baseURL,
		markedURL,
		cutoffDate,
		pageNum,
		pageSize,
		query,
		p.DB,
	)
}
