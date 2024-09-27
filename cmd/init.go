package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/queue"
)

var logFolderName = "logs"

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

// loadUrlsToQueue fetches all urls from URL model and loads them to queue
func loadUrlsToQueue(q *queue.UniqueQueue, m *models.Models, updateInterval uint) int {
	dburls, err := m.URLs.GetAll("is_monitored")
	if err != nil {
		fmt.Println(err)
	}
	intervalDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", updateInterval*24))
	currentTime := time.Now()
	// if isMonitored true and timestamp after updateInterval in db, set them as true, others false to not process
	for _, urlDB := range dburls {
		q.PushForce(urlDB.URL)
		expiryTime := urlDB.LastSaved.Add(intervalDuration)
		if urlDB.IsMonitored && (currentTime.After(expiryTime) || currentTime.Equal(expiryTime)) {
			q.SetMapValue(urlDB.URL, true)
		}
	}
	return len(dburls)
}
