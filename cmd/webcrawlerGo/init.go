package main

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/0x00f00bar/webcrawlerGo/models"
	"github.com/0x00f00bar/webcrawlerGo/queue"
)

func init() {
	// make a folder to store logs
	internal.CreateDirIfNotExists(logFolderName)
}

// loadUrlsToQueue fetches all urls from URL model and loads them to queue.
// Returns the number of URLs pushed to queue
func loadUrlsToQueue(
	ctx context.Context,
	baseURL url.URL,
	q *queue.UniqueQueue,
	m models.URLModel,
	updateInterval int,
	loggers *loggers,
	markedURLs []string,
) (int, error) {
	dburls, err := m.GetAll("is_monitored")
	if err != nil {
		return 0, err
	}
	intervalDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", updateInterval*24))
	currentTime := time.Now()
	var urlsPushedToQ int = 0
	// if isMonitored true and timestamp after updateInterval in db, set them as true, others false to not process
	for _, urlDB := range dburls {
		select {
		case <-ctx.Done():
			return urlsPushedToQ, nil
		default:

			parsedUrlDB, err := url.Parse(urlDB.URL)
			if err != nil {
				loggers.multiLogger.Printf("Unable to parse url '%s' from db\n", urlDB.URL)
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
						return 0, fmt.Errorf("unable to update url '%s': %v", urlDB.URL, err)
					}

				// else just add to map with false value to not access that URL
				default:
					fetchContent = false
				}

				if fetchContent {
					q.InsertForce(urlDB.URL)
					q.SetMapValue(urlDB.URL, true)
					urlsPushedToQ += 1
				} else {
					q.SetMapValue(urlDB.URL, false)
				}
			}
		}
	}
	return urlsPushedToQ, nil
}
