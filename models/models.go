package models

import (
	"context"
	"errors"
	"net/url"
	"time"
)

var (
	ErrRecordNotFound = errors.New("models: record not found")
	ErrNullURL        = errors.New("models: url cannot be empty or null")
	ErrEditConflict   = errors.New("models: edit conflict")
	ErrInvalidOrderBy = errors.New("models: invalid order by")
)

// DefaultDBTimeout for sql queries
var DefaultDBTimeout = 5 * time.Second

// QueryArgStr is the substring used in sql queries as placeholder
// for query arguments
const QueryArgStr = "__ARG__"

// Models embeds URLModel and PageModel interface
type Models struct {
	URLs  URLModel
	Pages PageModel
}

type URLModel interface {
	GetAll(orderBy string) ([]*URL, error)
	GetAllMonitored(orderBy string) ([]*URL, error)
	GetById(id int) (*URL, error)
	GetByURL(url string) (*URL, error)
	Insert(*URL) error
	Update(*URL) error
	Delete(id int) error
}

type PageModel interface {
	GetById(id int) (*Page, error)
	GetAllByURL(urlId uint, orderBy string) ([]*Page, error)
	GetLatestPageCount(
		ctx context.Context,
		baseURL *url.URL,
		markedURL string,
		cutoffDate time.Time,
	) (int, error)
	GetLatestPagesPaginated(
		ctx context.Context,
		baseURL *url.URL,
		markedURL string,
		cutoffDate time.Time,
		pageNum int,
		pageSize int,
	) ([]*PageContent, error)
	Insert(*Page) error
	// Update method is not required, yet
	// Update(*Page) error
	Delete(id int) error
}
