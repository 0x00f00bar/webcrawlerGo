package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	webcrawler "github.com/0x00f00bar/webcrawlerGo"
	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/0x00f00bar/webcrawlerGo/queue"
	tea "github.com/charmbracelet/bubbletea"
)

func beginCrawl(
	ctx context.Context,
	cmdArgs *cmdFlags,
	quit chan os.Signal,
	q *queue.UniqueQueue,
	m *models.Models,
	loggers *loggers,
) error {
	loadedURLs, err := initQueue(ctx, q, cmdArgs, m, loggers)
	if err != nil {
		// loggers.multiLogger.Println(err)
		exitCode = 1
		return err
	}
	loggers.multiLogger.Printf("Loaded %d URLs from model\n", loadedURLs)

	// display min of 5 log messages
	numMsgs := max(int(float32(*cmdArgs.nCrawlers)*float32(1.5)), 5)
	teaProg := tea.NewProgram(newteaProgModel(numMsgs, quit))

	prettyLogger := &crawLogger{
		teaProgram:   teaProg,
		crawlerCount: *cmdArgs.nCrawlers,
	}

	crawlerArmy, err := getCrawlerArmy(ctx, cmdArgs, q, m, loggers, prettyLogger)
	if err != nil {
		exitCode = 2
		return err
	}

	httpClient := getModifiedHTTPClient(maxIdleHttpConn)

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
	return nil
}

func initQueue(
	ctx context.Context,
	q *queue.UniqueQueue,
	cmdArgs *cmdFlags,
	m *models.Models,
	loggers *loggers,
) (int, error) {
	// insert base URL to URL model if not present
	// when present will throw unique constraint error, which can be ignored
	q.Insert(cmdArgs.baseURL.String())
	var t time.Time
	u := models.NewURL(cmdArgs.baseURL.String(), t, t, false)
	_ = m.URLs.Insert(u)

	// get all urls from db, put all in queue's map
	return loadUrlsToQueue(ctx, q, m.URLs, cmdArgs, loggers)
}

func getModifiedHTTPClient(maxIdleConn int) *http.Client {
	modifiedTransport := http.DefaultTransport.(*http.Transport).Clone()
	modifiedTransport.MaxIdleConnsPerHost = maxIdleConn

	httpClient := &http.Client{
		Timeout:   defaultTimeout,
		Transport: modifiedTransport,
	}
	return httpClient
}

func getCrawlerConfig(
	ctx context.Context,
	cmdArgs *cmdFlags,
	q *queue.UniqueQueue,
	m *models.Models,
	loggers *loggers,
	prettyLogger webcrawler.PrettyLogger,
) *webcrawler.CrawlerConfig {
	// if retry == 0, don't init request stats map
	var retryRequestStats map[string]int
	if *cmdArgs.retryTime > 0 {
		retryRequestStats = map[string]int{}
	}

	crawlerCfg := &webcrawler.CrawlerConfig{
		Queue:          q,
		Models:         m,
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

	return crawlerCfg
}

func getCrawlerArmy(
	ctx context.Context,
	cmdArgs *cmdFlags,
	q *queue.UniqueQueue,
	m *models.Models,
	loggers *loggers,
	prettyLogger webcrawler.PrettyLogger,
) ([]*webcrawler.Crawler, error) {
	crawlerCfg := getCrawlerConfig(ctx, cmdArgs, q, m, loggers, prettyLogger)

	// init n crawlers
	crawlerArmy, err := webcrawler.NNewCrawlers(*cmdArgs.nCrawlers, "crawler", crawlerCfg)
	if err != nil {
		exitCode = 2
		// loggers.multiLogger.Println(err)
		return nil, err
	}
	return crawlerArmy, nil
}
