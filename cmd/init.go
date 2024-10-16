package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/0x00f00bar/web-crawler/internal"
	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/queue"
)

func init() {
	// make a folder to store logs
	internal.CreateDirIfNotExists(logFolderName)
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
	return f, log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags|log.Lshortfile)
}

// loadUrlsToQueue fetches all urls from URL model and loads them to queue.
// Returns the number of URLs pushed to queue
func loadUrlsToQueue(
	ctx context.Context,
	baseURL url.URL,
	q *queue.UniqueQueue,
	m models.URLModel,
	updateInterval int,
	logger *log.Logger,
	markedURLs []string,
) int {
	dburls, err := m.GetAll("is_monitored")
	if err != nil {
		log.Println(err)
	}
	intervalDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", updateInterval*24))
	currentTime := time.Now()
	var urlsPushedToQ int = 0
	// if isMonitored true and timestamp after updateInterval in db, set them as true, others false to not process
	for _, urlDB := range dburls {
		select {
		case <-ctx.Done():
			return urlsPushedToQ
		default:

			parsedUrlDB, err := url.Parse(urlDB.URL)
			if err != nil {
				logger.Printf("Unable to parse url '%s' from model URLs\n", urlDB.URL)
			}
			// only process URLs belonging to baseURL
			if parsedUrlDB.Hostname() == baseURL.Hostname() {
				expiryTime := urlDB.LastSaved.Add(intervalDuration)

				var fetchContent bool

				switch {
				// add to queue if url is monitored and currentTime >= expiryTime
				case urlDB.IsMonitored &&
					(currentTime.After(expiryTime) || currentTime.Equal(expiryTime)):
					fetchContent = true

				// add to queue if url is marked by cmd args but not monitored
				case !urlDB.IsMonitored && internal.ContainsAny(urlDB.URL, markedURLs):
					fetchContent = true
					// mark url as monitored as if marked
					urlDB.IsMonitored = true
					err := m.Update(urlDB)
					if err != nil {
						logger.Fatalf("Unable to update model for url '%s': %v\n", urlDB.URL, err)
					}

				// else just add to map with false value to not access that URL
				default:
					fetchContent = false
				}

				if fetchContent {
					q.PushForce(urlDB.URL)
					q.SetMapValue(urlDB.URL, true)
					urlsPushedToQ += 1
				} else {
					q.SetMapValue(urlDB.URL, false)
				}
			}
		}
	}
	return urlsPushedToQ
}
