package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
)

type LogStreamChan chan crawlerStreamLog

type crawlerStreamLog struct {
	Time    string //timestamp
	Message string
}

func (l LogStreamChan) Log(message string) {
	select {
	case l <- crawlerStreamLog{Message: message, Time: time.Now().Format(time.RFC3339)}:
	default:
	}
}

func (l LogStreamChan) Quit() {
	close(l)
}

func (app *webapp) initiateCrawlHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BaseURL        string         `json:"baseurl"`      // -baseurl
		MarkedURLs     *string        `json:"murls"`        // -murls
		UpdateDaysPast *int           `json:"days"`         // -days
		DBDSN          *string        `json:"db-dsn"`       // -db-dsn
		IdleTimeout    *time.Duration `json:"idle-time"`    // -idle-time
		IgnorePattern  *string        `json:"ignore"`       // -ignore
		NCrawlers      *int           `json:"n"`            // -n
		ReqDelay       *time.Duration `json:"req-delay"`    // -req-delay
		RetryTime      *int           `json:"retry"`        // -retry
		UserAgent      *string        `json:"ua"`           // -ua
		UpdateHrefs    *bool          `json:"update-hrefs"` // -update-hrefs
	}

	if app.IsCrawling {
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

	v := internal.NewValidator()

	// validate baseurl
	parsedBaseURL, err := trimAndParseURL(&input.BaseURL)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("could not parse baseurl: %v", err))
		return
	}
	parsedCutOffDate, err := time.Parse(dateLayout, defaultCutOffDate)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	markedURLSlice := []string{}
	if input.MarkedURLs != nil {
		markedURLSlice = getMarkedURLS(*input.MarkedURLs)
	}
	if input.UpdateDaysPast == nil {
		*input.UpdateDaysPast = 1
	}
	if input.DBDSN == nil {
		*input.DBDSN = ""
	}
	if input.IdleTimeout == nil {
		*input.IdleTimeout = 10 * time.Second
	}
	if input.IgnorePattern == nil {
		*input.IgnorePattern = ""
	}
	if input.NCrawlers == nil {
		*input.NCrawlers = 10
	}
	if input.ReqDelay == nil {
		*input.ReqDelay = time.Millisecond * 50
	}
	if input.RetryTime == nil {
		*input.RetryTime = 2
	}
	if input.UserAgent == nil {
		*input.UserAgent = defaultUserAgent
	}
	if input.UpdateHrefs == nil {
		*input.UpdateHrefs = false
	}

	cmdArgs := cmdFlags{
		nCrawlers:      input.NCrawlers,
		baseURL:        parsedBaseURL,
		updateDaysPast: input.UpdateDaysPast,
		markedURLs:     markedURLSlice,
		ignorePattern:  seperateCmdArgs(*input.IgnorePattern),
		dbDSN:          input.DBDSN,
		userAgent:      input.UserAgent,
		reqDelay:       *input.ReqDelay,
		idleTimeout:    *input.IdleTimeout,
		retryTime:      input.RetryTime,
		savePath:       defaultSavePath,
		cutOffDate:     parsedCutOffDate,
		updateHrefs:    *input.UpdateHrefs,
	}

	validateFlags(v, &cmdArgs)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	ctx, cancel := context.WithCancel(app.OSSigCtx)

	app.CancelCrawl = cancel

	isProcessingChan := make(chan bool)

	go func() {
		app.IsCrawling = <-isProcessingChan
	}()

	app.IsCrawling = true
	app.StreamChan = make(LogStreamChan)
	go func() {
		// send false after Goexit or when crawlers are done
		defer func() {
			isProcessingChan <- false
			close(isProcessingChan)
		}()

		loadedURLs, err := initQueue(ctx, app.CrawlerQueue, &cmdArgs, app.Models, app.Loggers)
		if err != nil {
			app.Loggers.multiLogger.Printf("Error while initialising queue: %v\n", err)
			return
		}
		app.Loggers.multiLogger.Printf("Loaded %d URLs from model\n", loadedURLs)

		crawlerArmy, err := getCrawlerArmy(
			ctx,
			&cmdArgs,
			app.CrawlerQueue,
			app.Models,
			app.Loggers,
			app.StreamChan,
		)
		if err != nil {
			app.Loggers.multiLogger.Printf("Error while creating crawlers: %v\n", err)
			return
		}

		httpClient := getModifiedHTTPClient(maxIdleHttpConn)

		var wg sync.WaitGroup
		for _, crawler := range crawlerArmy {
			wg.Add(1)
			go func() {
				defer wg.Done()
				crawler.Crawl(httpClient)
			}()
		}
		wg.Wait()

		app.Loggers.multiLogger.Println("Done crawling")

	}()

	err = app.writeJSON(
		w,
		http.StatusAccepted,
		envelope{"status": "request accepted, crawling now"},
		nil,
	)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) cancelCrawlHandler(w http.ResponseWriter, r *http.Request) {
	if app.IsCrawling {
		app.CancelCrawl()
		err := app.writeJSON(
			w,
			http.StatusAccepted,
			envelope{"status": "previous crawl was cancelled"},
			nil,
		)
		if err != nil {
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err := app.writeJSON(w, http.StatusOK, envelope{"status": "not crawling"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) getStatusCrawlHandler(w http.ResponseWriter, r *http.Request) {
	err := app.writeJSON(w, http.StatusOK, envelope{"crawling": app.IsCrawling}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *webapp) streamCrawlerLogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	for logMsg := range app.StreamChan {
		fmt.Fprintf(w, "data: %s - %s", logMsg.Time, logMsg.Message)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
