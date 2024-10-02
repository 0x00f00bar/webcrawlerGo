package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/queue"
)

var (
	logFolderName         = "logs"
	dbMaxConnIdleDuration = 15 * time.Minute
	dbMaxOpenConn         = 25
	dbMaxIdleConn         = 25
)

func init() {
	// make a folder to store logs
	if _, err := os.Stat(logFolderName); os.IsNotExist(err) {
		err = os.Mkdir(logFolderName, 0755)
		if err != nil {
			panic(err)
		}
	}
}

// initialiseLogger returns a log file handle f and a MultiWriter logger (os.Stdout & f)
func initialiseLogger() (f *os.File, logger *log.Logger) {
	logFileName := fmt.Sprintf(
		"./%s/logfile-%s.log",
		logFolderName,
		time.Now().Format("02-01-2006-15-04-05"),
	)
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	return f, log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags)
}

// openDB opens and tests a connection to database identified
// by the dsn string (only psql for now)
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// setup db max connection idle time
	db.SetConnMaxIdleTime(dbMaxConnIdleDuration)

	// setup db max connections
	db.SetMaxOpenConns(dbMaxOpenConn)

	// setup db max idle connection
	db.SetMaxIdleConns(dbMaxIdleConn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// getMarkedURLS returns a slice of marked urls starting with '/'
func getMarkedURLS(cmdArg string) []string {
	markedURLs := []string{}
	cmdArg = strings.TrimSpace(cmdArg)
	// return empty slice when no marked urls
	if cmdArg == "" {
		return markedURLs
	}

	if strings.Contains(cmdArg, ",") {
		markedURLs = strings.Split(cmdArg, ",")
	} else {
		markedURLs = append(markedURLs, cmdArg)
	}

	// add leading '/' if not present
	for i, mUrl := range markedURLs {
		mUrl = strings.TrimSpace(mUrl)
		if mUrl[0] != '/' {
			mUrl = "/" + mUrl
		}
		markedURLs[i] = mUrl
	}

	return markedURLs
}

// loadUrlsToQueue fetches all urls from URL model and loads them to queue.
// Returns the number of URLs pushed to queue
func loadUrlsToQueue(
	baseURL url.URL,
	q *queue.UniqueQueue,
	m *models.Models,
	updateInterval int,
	logger *log.Logger,
) int {
	dburls, err := m.URLs.GetAll("is_monitored")
	if err != nil {
		fmt.Println(err)
	}
	intervalDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", updateInterval*24))
	currentTime := time.Now()
	var urlsPushedToQ int = 0
	// if isMonitored true and timestamp after updateInterval in db, set them as true, others false to not process
	for _, urlDB := range dburls {
		parsedUrlDB, err := url.Parse(urlDB.URL)
		if err != nil {
			logger.Printf("unable to parse url '%s' from model URLs\n", urlDB.URL)
		}
		// only process URLs belonging to baseURL
		if parsedUrlDB.Hostname() == baseURL.Hostname() {
			expiryTime := urlDB.LastSaved.Add(intervalDuration)
			// only add to queue if url is monitored and currentTime >= expiryTime
			// else just add to map with false value to not access that URL
			if urlDB.IsMonitored &&
				(currentTime.After(expiryTime) || currentTime.Equal(expiryTime)) {
				q.PushForce(urlDB.URL)
				q.SetMapValue(urlDB.URL, true)
				urlsPushedToQ += 1
			} else {
				q.SetMapValue(urlDB.URL, false)
			}
		}
	}
	return urlsPushedToQ
}
