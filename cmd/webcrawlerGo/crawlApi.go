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
	Time    string // timestamp
	Message string
}

func (l LogStreamChan) Log(message string) {
	select {
	case l <- crawlerStreamLog{Message: message, Time: time.Now().Format(time.RFC3339)}:
		fmt.Println("1")
	default:
		fmt.Println("2")
	}
}

func (l LogStreamChan) Quit() {
	l.Log("Done crawling")
	close(l)
}

func (app *webapp) initiateCrawlHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BaseURL        string  `json:"baseurl"`      // -baseurl
		MarkedURLs     *string `json:"murls"`        // -murls
		UpdateDaysPast *int    `json:"days"`         // -days
		DBDSN          *string `json:"db-dsn"`       // -db-dsn
		IdleTimeout    *string `json:"idle-time"`    // -idle-time
		IgnorePattern  *string `json:"ignore"`       // -ignore
		NCrawlers      *int    `json:"n"`            // -n
		ReqDelay       *string `json:"req-delay"`    // -req-delay
		RetryTime      *int    `json:"retry"`        // -retry
		UserAgent      *string `json:"ua"`           // -ua
		UpdateHrefs    *bool   `json:"update-hrefs"` // -update-hrefs
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
		input.UpdateDaysPast = new(int)
		*input.UpdateDaysPast = 1
	}
	if input.DBDSN == nil {
		input.DBDSN = new(string)
		*input.DBDSN = ""
	}
	if input.IdleTimeout == nil {
		input.IdleTimeout = new(string)
		*input.IdleTimeout = "10s"
	}
	if input.IgnorePattern == nil {
		input.IgnorePattern = new(string)
		*input.IgnorePattern = ""
	}
	if input.NCrawlers == nil {
		input.NCrawlers = new(int)
		*input.NCrawlers = 10
	}
	if input.ReqDelay == nil {
		input.ReqDelay = new(string)
		*input.ReqDelay = "50ms"
	}
	if input.RetryTime == nil {
		input.RetryTime = new(int)
		*input.RetryTime = 2
	}
	if input.UserAgent == nil {
		input.UserAgent = new(string)
		*input.UserAgent = defaultUserAgent
	}
	if input.UpdateHrefs == nil {
		input.UpdateHrefs = new(bool)
		*input.UpdateHrefs = false
	}

	requestDelay, err := time.ParseDuration(*input.ReqDelay)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("could not parse req-delay: %v", err))
		return
	}
	idleTime, err := time.ParseDuration(*input.IdleTimeout)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("could not parse idle-time: %v", err))
		return
	}

	cmdArgs := cmdFlags{
		nCrawlers:      input.NCrawlers,
		baseURL:        parsedBaseURL,
		updateDaysPast: input.UpdateDaysPast,
		markedURLs:     markedURLSlice,
		ignorePattern:  seperateCmdArgs(*input.IgnorePattern),
		dbDSN:          input.DBDSN,
		userAgent:      input.UserAgent,
		reqDelay:       requestDelay,
		idleTimeout:    idleTime,
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
	logCmdArgs(&cmdArgs, app.Loggers.fileLogger.Writer())

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
			msg := fmt.Sprintf("Error while initialising queue: %v\n", err)
			app.StreamChan.Log(msg)
			app.Loggers.multiLogger.Print(msg)
			return
		}
		msg := fmt.Sprintf("Loaded %d URLs from model\n", loadedURLs)
		app.StreamChan.Log(msg)
		app.Loggers.multiLogger.Print(msg)

		crawlerArmy, err := getCrawlerArmy(
			ctx,
			&cmdArgs,
			app.CrawlerQueue,
			app.Models,
			app.Loggers,
			app.StreamChan,
		)
		if err != nil {
			msg := fmt.Sprintf("Error while creating crawlers: %v\n", err)
			app.StreamChan.Log(msg)
			app.Loggers.multiLogger.Print(msg)
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "text/event-stream")

	clientGone := r.Context().Done()
	// var flusher http.Flusher
	rc := http.NewResponseController(w)
	// flusher, ok := w.(http.Flusher)
	// if !ok {
	// 	app.serverErrorResponse(w, r, fmt.Errorf("streaming unsupported"))
	// 	return
	// }

	for {
		select {
		case <-clientGone:
			fmt.Println("Client disconnected")
			return
		case <-app.StreamChan:
			logMsg := <-app.StreamChan
			fmt.Fprintf(w, "data: %s - %s\n\n", logMsg.Time, logMsg.Message)
			rc.Flush()
			// flusher.Flush()
		}
	}
}
