package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	version = "0.8.9"
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

	var exitCode int
	defer func() {
		os.Exit(exitCode)
	}()

	v := internal.NewValidator()

	// init file and os.Stdout logger
	f, loggers := initialiseLoggers()
	defer f.Close()
	f.Write([]byte(banner + "\n" + "v" + version + "\n\n"))

	// parse cmd flags; exit if flags invalid
	cmdArgs := parseCmdFlags(v, f)

	// init and test db
	driverName, dbConns, err := getDBConnections(*cmdArgs.dbDSN, loggers)
	if err != nil {
		exitCode = 1
		loggers.fileLogger.Println(err)
		return
	}
	loggers.multiLogger.Println("DB connection OK.")
	defer dbConns.Close()

	// if the driver used is sqlite3 consolidate the WAL journal to db
	// before closing connection
	defer sqlite.ExecWALCheckpoint(driverName, dbConns.writer)

	// create cancel context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// init models
	var m models.Models

	// get postgres models and initialise database tables
	if driverName == psql.DriverNamePgSQL {
		psqlModels := psql.NewPsqlDB(dbConns.writer)
		err := psqlModels.InitDatabase(ctx, dbConns.writer)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Println(err)
			return
		}
		m.URLs = psqlModels.URLModel
		m.Pages = psqlModels.PageModel
	}
	// get sqlite3 models and initialise database tables
	if driverName == sqlite.DriverNameSQLite {
		sqliteModels := sqlite.NewSQLiteDB(dbConns.reader, dbConns.writer)
		err := sqliteModels.InitDatabase(ctx, dbConns.writer)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Println(err)
			return
		}
		m.URLs = sqliteModels.URLModel
		m.Pages = sqliteModels.PageModel
	}

	// init queue & push base url
	q := queue.NewQueue()
	q.Insert(cmdArgs.baseURL.String())

	// chanel to get os signals
	quit := make(chan os.Signal, 1)

	go listenForSignals(cancel, quit, q, loggers)

	if cmdArgs.dbToDisk {
		err = saveDbContentToDisk(ctx, m.Pages, cmdArgs, cmdArgs.markedURLs, loggers)
		if err != nil {
			exitCode = 1
			loggers.multiLogger.Printf("Error while saving to disk: %v\n", err)
		} else {
			loggers.multiLogger.Println("Transfer completed")
		}
		return
	}

	// insert base URL to URL model if not present
	// when present will throw unique constraint error, which can be ignored
	var t time.Time
	u := models.NewURL(cmdArgs.baseURL.String(), t, t, false)
	_ = m.URLs.Insert(u)

	// get all urls from db, put all in queue's map
	loadedURLs, err := loadUrlsToQueue(ctx, q, m.URLs, cmdArgs, loggers)
	if err != nil {
		exitCode = 1
		loggers.multiLogger.Println(err)
		return
	}
	loggers.multiLogger.Printf("Loaded %d URLs from model\n", loadedURLs)

	// if retry == 0, don't init request stats map
	var retryRequestStats map[string]int
	if *cmdArgs.retryTime > 0 {
		retryRequestStats = map[string]int{}
	} else {
		retryRequestStats = nil
	}
	teaProg := tea.NewProgram(newteaProgModel(int(float32(*cmdArgs.nCrawlers)*float32(1.5)), quit))

	prettyLogger := &crawLogger{
		teaProgram:   teaProg,
		crawlerCount: *cmdArgs.nCrawlers,
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
		Logger:         loggers.fileLogger,
		RetryTimes:     *cmdArgs.retryTime,
		FailedRequests: retryRequestStats,
		Ctx:            ctx,
		PrettyLogger:   prettyLogger,
	}

	// init n crawlers
	crawlerArmy, err := webcrawler.NNewCrawlers(*cmdArgs.nCrawlers, "crawler", crawlerCfg)
	if err != nil {
		exitCode = 1
		loggers.multiLogger.Println(err)
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

	if _, err := teaProg.Run(); err != nil {
		fmt.Println("Error running program:", err)
	}

	// wait for crawlers
	wg.Wait()
	loggers.fileLogger.Println("Done")
	fmt.Println(redStyle.Margin(0, 0, 1, 2).Render("Done"))
}
