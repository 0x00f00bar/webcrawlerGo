package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/julienschmidt/httprouter"
)

const serverPort = 8100

type webapp struct {
	Models           *models.Models
	Loggers          *loggers
	OSSigCtx         context.Context // Context for OS Signals (SIGINT,SIGTERM)
	IsSavingToDisk   bool            // flag to verify if save2disk func is running
	CancelSaveToDisk context.CancelFunc
	IsCrawling       bool // to check if presently crawling
	CancelCrawl      context.CancelFunc
}

func (app *webapp) serve(ctx context.Context, quitChan chan os.Signal) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", serverPort),
		Handler:      app.routes(),
		ErrorLog:     app.Loggers.multiLogger,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // test with load
	}

	shutdownErr := make(chan error)

	go func() {
		s := <-quitChan
		app.Loggers.multiLogger.Printf("shutting down server. signal: %s\n", s.String())
		shutdownErr <- srv.Shutdown(ctx)
	}()

	app.Loggers.multiLogger.Printf("starting server on %s", srv.Addr)

	// go func() {
	// 	time.Sleep(time.Second)
	// 	app.openBrowser(fmt.Sprintf("http://localhost:%d", serverPort))
	// }()

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownErr
	if err != nil {
		return err
	}

	app.Loggers.multiLogger.Println("stopped server")
	return nil
}

func (app *webapp) routes() http.Handler {
	router := httprouter.New()
	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/v1/url", app.listURLHandler)
	router.HandlerFunc(http.MethodGet, "/v1/url/:id", app.getURLByIdHandler)
	router.HandlerFunc(http.MethodPost, "/v1/url", app.createURLHandler)
	router.HandlerFunc(http.MethodPatch, "/v1/url/:id", app.updateURLHandler)

	router.HandlerFunc(http.MethodGet, "/v1/page", app.listPageHandler)
	router.HandlerFunc(http.MethodGet, "/v1/page/:id", app.getPageByIdHandler)

	return app.logRequestMiddleware(router)
}
