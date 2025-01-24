package main

import (
	"errors"
	"net/http"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
)

func (app *webapp) getPageByIdHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	page, err := app.Models.Pages.GetById(int(id))
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"page": page}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) listPageHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URLId int
		models.CommonFilters
	}

	v := internal.NewValidator()
	qs := r.URL.Query()

	input.URLId = app.readInt(qs, "url_id", 0, v)

	input.CommonFilters.Page = app.readInt(qs, "page", 1, v)
	input.CommonFilters.PageSize = app.readInt(qs, "page_size", 10, v)
	input.CommonFilters.Sort = app.readString(qs, "sort", "id")
	var safeSortList []string
	safeSortList = append(safeSortList, models.PageColumns...)
	safeSortList = append(safeSortList, internal.PrefixString(models.PageColumns, "-")...)
	input.CommonFilters.SortSafeList = safeSortList

	if models.ValidateCommonFilters(v, input.CommonFilters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	pages, err := app.Models.Pages.GetAllByURL(uint(input.URLId), input.CommonFilters)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrInvalidOrderBy):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if pages == nil {
		pages = []*models.Page{}
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"page_list": pages}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
