package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	webcrawler "github.com/0x00f00bar/webcrawlerGo"
	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/0x00f00bar/webcrawlerGo/models/psql"
	"github.com/0x00f00bar/webcrawlerGo/models/sqlite"
	"github.com/0x00f00bar/webcrawlerGo/queue"
)

var (
	version = "0.7.0"
	banner  = `
                __                             __          ______    
 _      _____  / /_  ______________ __      __/ /__  _____/ ____/___ 
| | /| / / _ \/ __ \/ ___/ ___/ __ '/ | /| / / / _ \/ ___/ / __/ __ \
| |/ |/ /  __/ /_/ / /__/ /  / /_/ /| |/ |/ / /  __/ /  / /_/ / /_/ /
|__/|__/\___/_.___/\___/_/   \__,_/ |__/|__/_/\___/_/   \____/\____/ 
                                                                     `
)

func main() {
	printBanner()

	v := internal.NewValidator()

	// parse cmd flags; exit if flags invalid
	cmdArgs := pargeCmdFlags(v)

	var exitCode int
	defer func() {
		os.Exit(exitCode)
	}()

	// init file and os.Stdout logger
	f, logger := initialiseLogger()
	defer f.Close()

	// init and test db
	driverName, dbConns, err := getDBConnections(*cmdArgs.dbDSN, logger)
	if err != nil {
		exitCode = 1
		logger.Println(err)
		return
	}
	logger.Println("DB connection OK.")
	defer closeDBConns(dbConns)

	// if the driver used is sqlite3 consolidate the WAL journal to db
	// before closing connection
	defer func() {
		if driverName == driverNameSQLite {
			dbWriter := dbConns[1]
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			dbWriter.ExecContext(ctx, "PRAGMA wal_checkpoint(FULL);")
		}
	}()

	// init models
	var m models.Models

	// get postgres models
	if driverName == driverNamePgSQL {
		psqlModels := psql.NewPsqlDB(dbConns[0])
		m.URLs = psqlModels.URLModel
		m.Pages = psqlModels.PageModel
	}
	// get sqlite3 models
	if driverName == driverNameSQLite {
		sqliteModels := sqlite.NewSQLiteDB(dbConns[0], dbConns[1])
		m.URLs = sqliteModels.URLModel
		m.Pages = sqliteModels.PageModel
	}

	// init queue & push base url
	q := queue.NewQueue()
	q.Push(cmdArgs.baseURL.String())

	// create cancel context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	go listenForSignals(cancel, q, logger)

	if cmdArgs.dbToDisk {
		err = saveDbContentToDisk(
			ctx,
			m.Pages,
			cmdArgs.baseURL,
			cmdArgs.savePath,
			cmdArgs.cutOffDate,
			cmdArgs.markedURLs,
			logger,
		)
		if err != nil {
			exitCode = 1
			logger.Printf("Error while saving to disk: %v\n", err)
		} else {
			logger.Println("Transfer completed")
		}
		return
	}

	// insert base URL to URL model if not present
	// when present will throw unique constraint error, which can be ignored
	var t time.Time
	u := models.NewURL(cmdArgs.baseURL.String(), t, t, false)
	_ = m.URLs.Insert(u)

	// get all urls from db, put all in queue's map
	loadedURLs, err := loadUrlsToQueue(
		ctx,
		*cmdArgs.baseURL,
		q,
		m.URLs,
		*cmdArgs.updateDaysPast,
		logger,
		cmdArgs.markedURLs,
	)
	if err != nil {
		exitCode = 1
		logger.Println(err)
		return
	}
	logger.Printf("Loaded %d URLs from model\n", loadedURLs)

	// if retry == 0, don't init request stats map
	var retryRequestStats map[string]int
	if *cmdArgs.retryTime > 0 {
		retryRequestStats = map[string]int{}
	} else {
		retryRequestStats = nil
	}

	crawlerCfg := &webcrawler.CrawlerConfig{
		Queue:          q,
		Models:         &m,
		BaseURL:        cmdArgs.baseURL,
		UserAgent:      *cmdArgs.userAgent,
		MarkedURLs:     cmdArgs.markedURLs,
		IgnorePatterns: cmdArgs.ignorePattern,
		RequestDelay:   cmdArgs.reqDelay,
		IdleTimeout:    cmdArgs.idleTimeout,
		Log:            logger,
		RetryTimes:     *cmdArgs.retryTime,
		FailedRequests: retryRequestStats,
		Ctx:            ctx,
	}

	// init n crawlers
	crawlerArmy, err := webcrawler.NNewCrawlers(*cmdArgs.nCrawlers, "crawler", crawlerCfg)
	if err != nil {
		exitCode = 1
		logger.Println(err)
		return
	}

	modifiedTransport := http.DefaultTransport.(*http.Transport).Clone()
	modifiedTransport.MaxIdleConnsPerHost = 50

	httpClient := &http.Client{
		Timeout:   defaultTimeout,
		Transport: modifiedTransport,
	}

	// init waitgroup
	var wg sync.WaitGroup

	for _, crawler := range crawlerArmy {
		wg.Add(1)

		go func() {
			defer wg.Done()
			crawler.Crawl(httpClient)
		}()

	}

	// wait for crawlers
	wg.Wait()
	logger.Println("Done")
	fmt.Print(Red, "Done", Reset, "\n")
}