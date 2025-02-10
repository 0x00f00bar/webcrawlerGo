package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
)

func (app *webapp) initiateSaveDBContentHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BaseURL    string   `json:"baseurl"`
		CutOffDate string   `json:"cutoff_date"`
		MarkedURLs []string `json:"marked_urls"`
	}

	if app.IsSavingToDisk {
		headers := make(http.Header)
		headers.Set("Retry-After", "60")
		err := app.writeJSON(
			w,
			http.StatusServiceUnavailable,
			envelope{"error": "previous request is still running"},
			headers,
		)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validate baseurl
	baseURL, err := url.Parse(input.BaseURL)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("could not parse base_url: %v", err))
		return
	}
	v := internal.NewValidator()

	validateBaseURL(v, *baseURL)

	// validate CutOffDate
	v.Check(input.CutOffDate != "", "cutoff_date", "must be provided in format (YYYY-MM-DD)")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	parsedCutOffDate, err := time.Parse(dateLayout, input.CutOffDate)
	if err != nil {
		app.badRequestResponse(
			w,
			r,
			fmt.Errorf("could not parse cutoff_date (YYYY-MM-DD): %v", err),
		)
		return
	}

	savePath := fmt.Sprintf("./OUT/%s", time.Now().Format("2006-01-02_15-04-05"))

	ctx, cancel := context.WithCancel(app.OSSigCtx)

	app.CancelSaveToDisk = cancel

	isProcessingChan := make(chan bool)

	go func() {
		app.IsSavingToDisk = <-isProcessingChan
	}()

	app.IsSavingToDisk = true
	go func() {
		err = saveDbContentToDisk(
			ctx,
			app.Models.Pages,
			baseURL,
			savePath,
			parsedCutOffDate,
			input.MarkedURLs,
			app.Loggers,
		)
		if err != nil {
			app.Loggers.multiLogger.Printf("Error while saving to disk: %v\n", err)
		}
		isProcessingChan <- false
	}()

	msg := fmt.Sprintf("request accepted, saving files to: %s", savePath)
	err = app.writeJSON(w, http.StatusAccepted, envelope{"status": msg}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) cancelSaveDBContentHandler(w http.ResponseWriter, r *http.Request) {

	if app.IsSavingToDisk {
		app.CancelSaveToDisk()
		err := app.writeJSON(
			w,
			http.StatusOK,
			envelope{"status": "previous request was cancelled"},
			nil,
		)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err := app.writeJSON(w, http.StatusOK, envelope{"status": "no request running"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
