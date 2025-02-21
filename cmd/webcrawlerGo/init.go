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
	q *queue.UniqueQueue,
	m models.URLModel,
	cmdArgs *cmdFlags,
	loggers *loggers,
	urls ...string,
) (int, error) {
	uf := models.URLFilter{}
	cf := models.CommonFilters{
		Page:         1,
		PageSize:     100000,
		Sort:         "is_monitored",
		SortSafeList: models.URLColumns,
	}
	dburls, _, err := m.GetAll(uf, cf)
	if err != nil {
		return 0, err
	}

	UrlListPresent := len(urls) > 0
	intervalDuration, _ := time.ParseDuration(fmt.Sprintf("%dh", *cmdArgs.updateDaysPast*24))
	currentTime := time.Now()
	var totalUrlsPushedToQ int = 0
	// if isMonitored true and timestamp after updateInterval in db, set them as true, others false to not process
	for _, urlDB := range dburls {
		select {
		case <-ctx.Done():
			return totalUrlsPushedToQ, nil
		default:
			// skip dead urls
			// crawler will never crawl a dead url,
			// manually set the URL alive if the URL is back online
			if !urlDB.IsAlive {
				q.SetMapValue(urlDB.URL, false)
				continue
			}

			// skip urls containing ignored patterns
			if internal.ContainsAny(urlDB.URL, cmdArgs.ignorePattern) {
				continue
			}

			parsedUrlDB, err := url.Parse(urlDB.URL)
			if err != nil {
				loggers.multiLogger.Printf("Unable to parse url '%s' from db\n", urlDB.URL)
				continue
			}
			// only process URLs belonging to baseURL
			if parsedUrlDB.Hostname() == cmdArgs.baseURL.Hostname() {
				expiryTime := urlDB.LastSaved.Add(intervalDuration)

				var fetchContent bool

				switch {
				// if UrlListPresent then just add to known URL
				case UrlListPresent:
					fetchContent = false

				// add to queue if url is monitored and currentTime >= expiryTime
				case urlDB.IsMonitored &&
					(currentTime.After(expiryTime) || currentTime.Equal(expiryTime)):
					fetchContent = true

				// add to queue if url is marked by cmd args but not monitored
				case !urlDB.IsMonitored && internal.ContainsAny(urlDB.URL, cmdArgs.markedURLs):
					fetchContent = true
					// mark url as monitored as if marked
					urlDB.IsMonitored = true
					err := m.Update(urlDB)
					if err != nil {
						return 0, fmt.Errorf("unable to update url '%s': %v", urlDB.URL, err)
					}

				// else just add to map with false value to not save the URL content
				default:
					fetchContent = false
				}

				if fetchContent {
					q.InsertForce(urlDB.URL)
					q.SetMapValue(urlDB.URL, fetchContent)
					totalUrlsPushedToQ += 1
				} else if cmdArgs.updateHrefs {
					q.InsertForce(urlDB.URL)
					totalUrlsPushedToQ += 1
				} else {
					q.SetMapValue(urlDB.URL, fetchContent)
				}
			}
		}
	}

	// add urls from urls list
	for _, urlItem := range urls {
		// skip urls containing ignored patterns
		if internal.ContainsAny(urlItem, cmdArgs.ignorePattern) {
			continue
		}

		parsedUrl, err := url.Parse(urlItem)
		if err != nil {
			loggers.multiLogger.Printf("Unable to parse url '%s' from url list\n", urlItem)
			continue
		}
		// only process URLs belonging to baseURL
		if parsedUrl.Hostname() == cmdArgs.baseURL.Hostname() {
			q.InsertForce(urlItem)
			q.SetMapValue(urlItem, true)
			totalUrlsPushedToQ += 1
		}
	}

	return totalUrlsPushedToQ, nil
}
