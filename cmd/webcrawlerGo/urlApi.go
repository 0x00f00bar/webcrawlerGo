package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
)

func (app *webapp) createURLHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	var t time.Time
	url := models.NewURL(input.URL, t, t, true)

	v := internal.NewValidator()

	if models.ValidateURL(v, url); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.Models.URLs.Insert(url)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			app.errorResponse(
				w,
				r,
				http.StatusConflict,
				fmt.Sprintf("url '%s' is already present", url.URL),
			)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/url/%d", url.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"url": url}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) getURLByIdHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	url, err := app.Models.URLs.GetById(int(id))
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"url": url}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) updateURLHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// pointer type to identify if fields present in JSON
	// json items with null values will be ignored and
	// will remain unchanged
	var input struct {
		IsMonitored *bool `json:"is_monitored"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	url, err := app.Models.URLs.GetById(int(id))
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if input.IsMonitored != nil {
		url.IsMonitored = *input.IsMonitored
	}

	v := internal.NewValidator()

	if models.ValidateURL(v, url); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.Models.URLs.Update(url)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"url": url}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) listURLHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		models.URLFilter
		models.CommonFilters
	}

	v := internal.NewValidator()
	qs := r.URL.Query()

	input.URLFilter.URL = app.readString(qs, "url", "")
	input.IsMonitored, input.IsMonitoredPresent = app.readBool(qs, "is_monitored", v)
	input.IsAlive, input.IsAlivePresent = app.readBool(qs, "is_alive", v)

	input.CommonFilters.Page = app.readInt(qs, "page", 1, v)
	input.CommonFilters.PageSize = app.readInt(qs, "page_size", 10, v)
	input.CommonFilters.Sort = app.readString(qs, "sort", "id")
	var safeSortList []string
	safeSortList = append(safeSortList, models.URLColumns...)
	safeSortList = append(safeSortList, internal.PrefixString(models.URLColumns, "-")...)
	input.CommonFilters.SortSafeList = safeSortList

	if models.ValidateCommonFilters(v, input.CommonFilters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	urls, metaData, err := app.Models.URLs.GetAll(input.URLFilter, input.CommonFilters)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrInvalidOrderBy):
			app.badRequestResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if urls == nil {
		urls = []*models.URL{}
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"url_list": urls, "metadata": metaData}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
