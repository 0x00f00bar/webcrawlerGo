package models

import "errors"

var (
	ErrRecordNotFound = errors.New("models: record not found")
	ErrNullURL        = errors.New("models: urls cannot be empty or null")
	ErrEditConflict   = errors.New("models: edit conflict")
)

type Models struct {
	URLs  URLModel
	Pages PageModel
}

type URLModel interface {
	GetById(id int) (*URL, error)
	GetByURL(url string) (*URL, error)
	Insert(*URL) error
	Update(*URL) error
	Delete(int) error
}

type PageModel interface {
	GetById(id int) (*Page, error)
	GetByURL(url string) (*Page, error)
	Insert(*Page) error
	Update(*Page) error
	Delete(int) error
}
