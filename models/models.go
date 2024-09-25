package models

import "errors"

var (
	ErrRecordNotFound = errors.New("models: record not found")
	ErrNullURL        = errors.New("models: urls cannot be empty or null")
	ErrEditConflict   = errors.New("models: edit conflict")
	ErrInvalidOrderBy = errors.New("models: invalid order by")
)

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
	Insert(*Page) error
	// Update method is not required, yet
	// Update(*Page) error
	Delete(id int) error
}
