package models

type Models struct {
	Pages []*Page
	ULRs  []*URL
}

type URLModel interface {
	GetById(id int) (*URL, error)
	GetByURL(url string) (*URL, error)
	Insert(*URL) (*URL, error)
	Update(*URL) (*URL, error)
	Delete(*URL) error
}

type PageModel interface {
	GetById(id int) (*Page, error)
	GetByURL(url string) (*Page, error)
	Insert(*Page) (*Page, error)
	Update(*Page) (*Page, error)
	Delete(*Page) error
}
